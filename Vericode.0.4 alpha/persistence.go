package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// StoreObj is a file-backed key-value store. Values are VeriCode objects
// encoded as JSON, so only JSON-compatible values can be stored. The mutex
// makes one store safe to use from spawned tasks; snapshot isolation shares
// the store pointer so every task sees the same data.
type StoreObj struct {
	mu   sync.Mutex
	path string
	data map[string]Object
}

func (s *StoreObj) Type() ObjectType { return STORE_OBJ }

func (s *StoreObj) Inspect() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Sprintf("<store %s: %d keys>", s.path, len(s.data))
}

// persistenceBuiltins exposes the 0.6 persistence layer: small JSON documents
// on disk with get/set/delete access.
func persistenceBuiltins() []*Builtin {
	return []*Builtin{
		{
			Name: "db_open",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("db_open() takes 1 argument (path), got %d", len(args))
				}
				path, ok := args[0].(*String)
				if !ok {
					return newError("db_open() path must be a string, got %s", args[0].Type())
				}
				if !vm.allowIO {
					return newError("db_open() is disabled in the web IDE; run with the CLI (veri -run file.veri) to open a store")
				}
				store, err := openStore(path.Value)
				if err != nil {
					return newError("db_open() %v", err)
				}
				return store
			},
		},
		{
			Name: "db_get",
			Fn: func(vm *VM, args ...Object) Object {
				store, key, errObj := storeArgs("db_get", args)
				if errObj != nil {
					return errObj
				}
				store.mu.Lock()
				defer store.mu.Unlock()
				value, ok := store.data[key]
				if !ok {
					return &Null{}
				}
				return snapshotObject(value, make(map[Object]Object))
			},
		},
		{
			Name: "db_set",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 3 {
					return newError("db_set() takes 3 arguments (store, key, value), got %d", len(args))
				}
				store, key, errObj := storeAndKey("db_set", args)
				if errObj != nil {
					return errObj
				}
				value := args[2]
				if _, err := objectToJSON(value); err != nil {
					return newError("db_set() cannot store this value: %v", err)
				}
				store.mu.Lock()
				defer store.mu.Unlock()
				previous, existed := store.data[key]
				store.data[key] = snapshotObject(value, make(map[Object]Object))
				if err := store.saveLocked(); err != nil {
					if existed {
						store.data[key] = previous
					} else {
						delete(store.data, key)
					}
					return newError("db_set() %v", err)
				}
				return &Null{}
			},
		},
		{
			Name: "db_delete",
			Fn: func(vm *VM, args ...Object) Object {
				store, key, errObj := storeArgs("db_delete", args)
				if errObj != nil {
					return errObj
				}
				store.mu.Lock()
				defer store.mu.Unlock()
				previous, existed := store.data[key]
				if !existed {
					return &BooleanObj{Value: false}
				}
				delete(store.data, key)
				if err := store.saveLocked(); err != nil {
					store.data[key] = previous
					return newError("db_delete() %v", err)
				}
				return &BooleanObj{Value: true}
			},
		},
		{
			Name: "db_has",
			Fn: func(vm *VM, args ...Object) Object {
				store, key, errObj := storeArgs("db_has", args)
				if errObj != nil {
					return errObj
				}
				store.mu.Lock()
				defer store.mu.Unlock()
				_, ok := store.data[key]
				return &BooleanObj{Value: ok}
			},
		},
		{
			Name: "db_keys",
			Fn: func(vm *VM, args ...Object) Object {
				store, errObj := storeArg("db_keys", args)
				if errObj != nil {
					return errObj
				}
				store.mu.Lock()
				defer store.mu.Unlock()
				keys := make([]string, 0, len(store.data))
				for key := range store.data {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				elements := make([]Object, len(keys))
				for i, key := range keys {
					elements[i] = &String{Value: key}
				}
				return &Array{Elements: elements}
			},
		},
	}
}

func storeArg(name string, args []Object) (*StoreObj, Object) {
	if len(args) != 1 {
		return nil, newError("%s() takes 1 argument (store), got %d", name, len(args))
	}
	store, ok := args[0].(*StoreObj)
	if !ok {
		return nil, newError("%s() argument must be a store from db_open(), got %s", name, args[0].Type())
	}
	return store, nil
}

func storeAndKey(name string, args []Object) (*StoreObj, string, Object) {
	if len(args) < 2 {
		return nil, "", newError("%s() takes a store and a key", name)
	}
	store, ok := args[0].(*StoreObj)
	if !ok {
		return nil, "", newError("%s() first argument must be a store from db_open(), got %s", name, args[0].Type())
	}
	key, ok := args[1].(*String)
	if !ok {
		return nil, "", newError("%s() key must be a string, got %s", name, args[1].Type())
	}
	return store, key.Value, nil
}

func storeArgs(name string, args []Object) (*StoreObj, string, Object) {
	if len(args) != 2 {
		return nil, "", newError("%s() takes 2 arguments (store, key), got %d", name, len(args))
	}
	return storeAndKey(name, args)
}

// openStore reads an existing JSON object or starts an empty store. A new
// store is written immediately so permission problems surface at open time.
func openStore(path string) (*StoreObj, error) {
	data := map[string]Object{}
	created := false

	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		if strings.TrimSpace(string(raw)) != "" {
			value, decErr := decodeJSON(string(raw))
			if decErr != nil {
				return nil, fmt.Errorf("cannot read '%s': %v", path, decErr)
			}
			hash, ok := value.(*Hash)
			if !ok {
				return nil, fmt.Errorf("'%s' must contain a JSON object, found %s", path, value.Type())
			}
			for _, pair := range hash.Pairs {
				key, ok := pair.Key.(*String)
				if !ok {
					return nil, fmt.Errorf("'%s' has a non-string key", path)
				}
				data[key.Value] = pair.Value
			}
		}
	case os.IsNotExist(err):
		created = true
	default:
		return nil, fmt.Errorf("cannot read '%s': %v", path, err)
	}

	store := &StoreObj{path: path, data: data}
	if created {
		if err := store.saveLocked(); err != nil {
			return nil, err
		}
	}
	return store, nil
}

// saveLocked encodes the store and replaces the file atomically. The caller
// must hold s.mu.
func (s *StoreObj) saveLocked() error {
	pairs := make(map[string]HashPairObj, len(s.data))
	for key, value := range s.data {
		pairs[cellKey(key)] = HashPairObj{Key: &String{Value: key}, Value: value}
	}
	encoded, err := objectToJSON(&Hash{Pairs: pairs})
	if err != nil {
		return fmt.Errorf("cannot encode store: %v", err)
	}
	content, err := json.MarshalIndent(encoded, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot encode store: %v", err)
	}
	content = append(content, '\n')
	if err := writeFileAtomic(s.path, content); err != nil {
		return fmt.Errorf("cannot write '%s': %v", s.path, err)
	}
	return nil
}

// writeFileAtomic writes to a temporary file in the destination directory and
// renames it over the target, so a crash never leaves a partial store.
func writeFileAtomic(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		tmp.Close()
		os.Remove(tmpName)
	}
	if _, err := tmp.Write(content); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
