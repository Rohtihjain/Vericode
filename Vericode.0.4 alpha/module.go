package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// importState is one VM tree's module table: paths already loaded, the paths
// currently being loaded (for cycle detection), and the first free global
// slot for modules compiled at runtime.
type importState struct {
	nextGlobal int
	modules    map[string]Object // absolute path -> namespace hash
	loading    map[string]bool
}

func newImportState(nextGlobal int) *importState {
	return &importState{
		nextGlobal: nextGlobal,
		modules:    make(map[string]Object),
		loading:    make(map[string]bool),
	}
}

// freshImportState gives a child VM its own cache, starting allocations above
// every global slot already present in the snapshot it was created from.
func (vm *VM) freshImportState() *importState {
	base := 0
	if vm.imports != nil {
		base = vm.imports.nextGlobal
	}
	return newImportState(base)
}

// importError attaches the import site to a failure, keeping a module's own
// formatted diagnostic as the causal detail (and preserving fatal errors).
func (vm *VM) importError(path string, err error) error {
	if re, ok := err.(*runtimeErr); ok {
		if re.fatal {
			return err
		}
		return vm.newRuntimeError(false, "RuntimeError", "import '%s' failed\n\n%s", path, err.Error())
	}
	return vm.runtimeError("import '%s' failed: %v", path, err)
}

// resolveImportPath interprets a path relative to the file containing the
// import statement (a module function may run from another file).
func (vm *VM) resolveImportPath(rel string) string {
	if filepath.IsAbs(rel) {
		return filepath.Clean(rel)
	}
	base := "."
	if vm.frameCount > 0 {
		if f := vm.currentFrame(); f.cl != nil && f.cl.Fn != nil && f.cl.Fn.Filename != "" {
			base = filepath.Dir(f.cl.Fn.Filename)
		} else if vm.filename != "" && !strings.HasPrefix(vm.filename, "<") {
			base = filepath.Dir(vm.filename)
		}
	}
	return filepath.Clean(filepath.Join(base, rel))
}

// importModule loads, compiles, and runs a module once, returning a hash of its
// top-level globals. Module code shares this VM's global table (at slots above
// the caller's), output, args, input, and I/O setting, but runs in its own
// frame and source map so diagnostics point at the module file.
func (vm *VM) importModule(rel string) (Object, error) {
	if !vm.allowIO {
		return nil, fmt.Errorf("import is disabled in the web IDE; run with the CLI (veri -run file.veri) to import modules")
	}
	if vm.imports == nil {
		vm.imports = newImportState(0)
	}
	state := vm.imports

	abs, err := filepath.Abs(vm.resolveImportPath(rel))
	if err != nil {
		return nil, fmt.Errorf("cannot resolve path: %v", err)
	}
	if ns, ok := state.modules[abs]; ok {
		return ns, nil
	}
	if state.loading[abs] {
		return nil, fmt.Errorf("circular import of '%s'", rel)
	}
	state.loading[abs] = true
	defer delete(state.loading, abs)

	srcBytes, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("cannot read module: %v", err)
	}
	src := strings.TrimPrefix(string(srcBytes), "\xef\xbb\xbf")

	lexer := NewLexer(src)
	parser := NewParser(lexer)
	parser.SetFilename(abs)
	program := parser.ParseProgram()
	if errs := parser.Errors(); len(errs) > 0 {
		return nil, fmt.Errorf("parse error:\n%s", strings.Join(errs, "\n"))
	}

	base := state.nextGlobal
	compiler := NewCompilerWithSource(abs, src)
	compiler.globalBase = base
	if err := compiler.Compile(program); err != nil {
		return nil, err
	}
	bc := compiler.Bytecode()
	state.nextGlobal = base + len(bc.Symbols)

	modVM := &VM{
		constants:    bc.Constants,
		stack:        make([]Object, 65536),
		globals:      vm.globals, // shared: module code and its exports use these slots
		out:          vm.out,
		input:        vm.input,
		args:         vm.args,
		sourceMap:    bc.SourceMap,
		source:       bc.Source,
		filename:     abs,
		cancel:       make(chan struct{}),
		parentCancel: vm.cancel,
		killOnce:     &sync.Once{},
		allowIO:      vm.allowIO,
		detector:     vm.detector,
		vmID:         nextVMID(),
		imports:      state,
	}
	modVM.frames[0] = frame{ip: -1, instructions: bc.Instructions}
	modVM.frameCount = 1
	if modVM.detector != nil {
		modVM.detector.enter(modVM.vmID, fmt.Sprintf("module '%s'", rel))
	}
	if err := modVM.Run(); err != nil {
		return nil, err
	}

	ns := &Hash{Pairs: make(map[string]HashPairObj)}
	for i, name := range bc.Symbols {
		if strings.HasPrefix(name, "__") {
			continue
		}
		value := vm.globals[base+i]
		if value == nil {
			continue
		}
		key := &String{Value: name}
		ns.Pairs[key.HashKey()] = HashPairObj{Key: key, Value: value}
	}
	state.modules[abs] = ns
	return ns, nil
}
