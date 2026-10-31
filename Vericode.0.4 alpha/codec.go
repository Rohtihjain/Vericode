package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

// ─── CSV / JSON / files (0.4) ────────────────────────────────────────────────

func codecBuiltins() []*Builtin {
	return []*Builtin{
		{
			Name: "csv_parse",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("csv_parse() takes 1 argument (text), got %d", len(args))
				}
				text, ok := args[0].(*String)
				if !ok {
					return newError("csv_parse() argument must be a string, got %s", args[0].Type())
				}
				records, err := parseCSV(text.Value)
				if err != nil {
					return newError("csv_parse() %v", err)
				}
				out := make([]Object, len(records))
				for i, rec := range records {
					cells := make([]Object, len(rec))
					for j, cell := range rec {
						cells[j] = &String{Value: cell}
					}
					out[i] = &Array{Elements: cells}
				}
				return &Array{Elements: out}
			},
		},
		{
			Name: "csv_write",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("csv_write() takes 1 argument (table or rows), got %d", len(args))
				}
				rows, errObj := csvRows(args[0])
				if errObj != nil {
					return errObj
				}
				var b strings.Builder
				w := csv.NewWriter(&b)
				for _, row := range rows {
					if err := w.Write(row); err != nil {
						return newError("csv_write() %v", err)
					}
				}
				w.Flush()
				if err := w.Error(); err != nil {
					return newError("csv_write() %v", err)
				}
				return &String{Value: b.String()}
			},
		},
		{
			Name: "csv_table",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("csv_table() takes 1 argument (text), got %d", len(args))
				}
				text, ok := args[0].(*String)
				if !ok {
					return newError("csv_table() argument must be a string, got %s", args[0].Type())
				}
				records, err := parseCSV(text.Value)
				if err != nil {
					return newError("csv_table() %v", err)
				}
				if len(records) == 0 {
					return &Table{Headers: []string{}, Rows: []*Hash{}}
				}
				headers := records[0]
				seen := make(map[string]bool, len(headers))
				for _, h := range headers {
					if seen[h] {
						return newError("csv_table() duplicate column '%s'", h)
					}
					seen[h] = true
				}
				t := &Table{Headers: append([]string(nil), headers...), Rows: make([]*Hash, 0, len(records)-1)}
				for i, rec := range records[1:] {
					if len(rec) > len(headers) {
						return newError("csv_table() row %d has %d fields, header has %d", i+1, len(rec), len(headers))
					}
					pairs := make(map[string]HashPairObj, len(headers))
					for j, h := range headers {
						val := ""
						if j < len(rec) {
							val = rec[j]
						}
						pairs[cellKey(h)] = HashPairObj{Key: &String{Value: h}, Value: &String{Value: val}}
					}
					t.Rows = append(t.Rows, &Hash{Pairs: pairs})
				}
				return t
			},
		},
		{
			Name: "json_parse",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("json_parse() takes 1 argument (text), got %d", len(args))
				}
				text, ok := args[0].(*String)
				if !ok {
					return newError("json_parse() argument must be a string, got %s", args[0].Type())
				}
				obj, err := decodeJSON(text.Value)
				if err != nil {
					return newError("json_parse() %v", err)
				}
				return obj
			},
		},
		{
			Name: "json_write",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) < 1 || len(args) > 2 {
					return newError("json_write() takes 1-2 arguments (value, [indent]), got %d", len(args))
				}
				indent := int64(0)
				if len(args) == 2 {
					n, ok := args[1].(*Integer)
					if !ok {
						return newError("json_write() indent must be an integer, got %s", args[1].Type())
					}
					if n.Value < 0 {
						return newError("json_write() indent cannot be negative, got %d", n.Value)
					}
					indent = n.Value
				}
				v, err := objectToJSON(args[0])
				if err != nil {
					return newError("json_write() %v", err)
				}
				var b strings.Builder
				enc := json.NewEncoder(&b)
				enc.SetEscapeHTML(false)
				if indent > 0 {
					enc.SetIndent("", strings.Repeat(" ", int(indent)))
				}
				if err := enc.Encode(v); err != nil {
					return newError("json_write() %v", err)
				}
				return &String{Value: strings.TrimSuffix(b.String(), "\n")}
			},
		},
		{
			Name: "read_file",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("read_file() takes 1 argument (path), got %d", len(args))
				}
				path, ok := args[0].(*String)
				if !ok {
					return newError("read_file() path must be a string, got %s", args[0].Type())
				}
				if !vm.allowIO {
					return newError("read_file() is disabled in the web IDE; run with the CLI (veri -run file.veri) to read files")
				}
				data, err := os.ReadFile(path.Value)
				if err != nil {
					return newError("read_file() cannot read '%s': %v", path.Value, err)
				}
				return &String{Value: string(data)}
			},
		},
		{
			Name: "write_file",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 2 {
					return newError("write_file() takes 2 arguments (path, text), got %d", len(args))
				}
				path, ok := args[0].(*String)
				if !ok {
					return newError("write_file() path must be a string, got %s", args[0].Type())
				}
				text, ok := args[1].(*String)
				if !ok {
					return newError("write_file() text must be a string, got %s (use str() to convert)", args[1].Type())
				}
				if !vm.allowIO {
					return newError("write_file() is disabled in the web IDE; run with the CLI (veri -run file.veri) to write files")
				}
				if err := os.WriteFile(path.Value, []byte(text.Value), 0o644); err != nil {
					return newError("write_file() cannot write '%s': %v", path.Value, err)
				}
				return &Null{}
			},
		},
	}
}

// ─── CSV helpers ─────────────────────────────────────────────────────────────

func parseCSV(text string) ([][]string, error) {
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1
	return r.ReadAll()
}

// csvRows converts a table, an array of arrays, or an array of hashes into
// CSV records.
func csvRows(o Object) ([][]string, Object) {
	switch v := o.(type) {
	case *Table:
		return tableCSVRows(v), nil
	case *Array:
		if len(v.Elements) == 0 {
			return [][]string{}, nil
		}
		switch v.Elements[0].(type) {
		case *Array:
			rows := make([][]string, len(v.Elements))
			for i, el := range v.Elements {
				row, ok := el.(*Array)
				if !ok {
					return nil, newError("csv_write() element %d must be an array, got %s", i, el.Type())
				}
				rows[i] = make([]string, len(row.Elements))
				for j, cell := range row.Elements {
					rows[i][j] = csvCell(cell)
				}
			}
			return rows, nil
		case *Hash:
			tbl := tableFromHashes("csv_write", v.Elements, nil, false)
			if errObj, ok := tbl.(*Error); ok {
				return nil, errObj
			}
			return tableCSVRows(tbl.(*Table)), nil
		default:
			return nil, newError("csv_write() expects a table, an array of arrays, or an array of hashes, got an array of %s", v.Elements[0].Type())
		}
	default:
		return nil, newError("csv_write() expects a table, an array of arrays, or an array of hashes, got %s", o.Type())
	}
}

func tableCSVRows(t *Table) [][]string {
	rows := make([][]string, 0, len(t.Rows)+1)
	rows = append(rows, append([]string(nil), t.Headers...))
	for _, row := range t.Rows {
		rec := make([]string, len(t.Headers))
		for i, h := range t.Headers {
			rec[i] = csvCell(rowCell(row, h))
		}
		rows = append(rows, rec)
	}
	return rows
}

func csvCell(o Object) string {
	switch v := o.(type) {
	case *String:
		return v.Value
	case *Null:
		return ""
	default:
		return o.Inspect()
	}
}

// ─── JSON helpers ────────────────────────────────────────────────────────────

func decodeJSON(text string) (Object, error) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("empty input")
		}
		return nil, err
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("unexpected data after JSON value")
	}
	return jsonValueToObject(v)
}

func jsonValueToObject(v interface{}) (Object, error) {
	switch x := v.(type) {
	case nil:
		return &Null{}, nil
	case bool:
		return &BooleanObj{Value: x}, nil
	case string:
		return &String{Value: x}, nil
	case json.Number:
		s := string(x)
		if strings.ContainsAny(s, ".eE") {
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid number %s", s)
			}
			return &Float{Value: f}, nil
		}
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			f, ferr := strconv.ParseFloat(s, 64)
			if ferr != nil {
				return nil, fmt.Errorf("invalid number %s", s)
			}
			return &Float{Value: f}, nil
		}
		return &Integer{Value: i}, nil
	case []interface{}:
		out := make([]Object, len(x))
		for i, el := range x {
			obj, err := jsonValueToObject(el)
			if err != nil {
				return nil, err
			}
			out[i] = obj
		}
		return &Array{Elements: out}, nil
	case map[string]interface{}:
		pairs := make(map[string]HashPairObj, len(x))
		for k, val := range x {
			obj, err := jsonValueToObject(val)
			if err != nil {
				return nil, err
			}
			pairs[cellKey(k)] = HashPairObj{Key: &String{Value: k}, Value: obj}
		}
		return &Hash{Pairs: pairs}, nil
	default:
		return nil, fmt.Errorf("unsupported JSON value %T", v)
	}
}

func objectToJSON(o Object) (interface{}, error) {
	return objectToJSONInner(o, make(map[Object]bool))
}

func objectToJSONInner(o Object, visiting map[Object]bool) (interface{}, error) {
	switch v := o.(type) {
	case *Integer:
		return v.Value, nil
	case *Float:
		if math.IsNaN(v.Value) || math.IsInf(v.Value, 0) {
			return nil, fmt.Errorf("cannot encode %v as JSON", v.Value)
		}
		return v.Value, nil
	case *BooleanObj:
		return v.Value, nil
	case *String:
		return v.Value, nil
	case *Null:
		return nil, nil
	case *Array:
		if visiting[o] {
			return nil, fmt.Errorf("cannot encode a cyclic value to JSON")
		}
		visiting[o] = true
		defer delete(visiting, o)
		out := make([]interface{}, len(v.Elements))
		for i, el := range v.Elements {
			conv, err := objectToJSONInner(el, visiting)
			if err != nil {
				return nil, err
			}
			out[i] = conv
		}
		return out, nil
	case *Hash:
		if visiting[o] {
			return nil, fmt.Errorf("cannot encode a cyclic value to JSON")
		}
		visiting[o] = true
		defer delete(visiting, o)
		out := make(map[string]interface{}, len(v.Pairs))
		for _, pair := range v.Pairs {
			key, ok := pair.Key.(*String)
			if !ok {
				return nil, fmt.Errorf("hash keys must be strings, got %s", pair.Key.Type())
			}
			conv, err := objectToJSONInner(pair.Value, visiting)
			if err != nil {
				return nil, err
			}
			out[key.Value] = conv
		}
		return out, nil
	case *Table:
		if visiting[o] {
			return nil, fmt.Errorf("cannot encode a cyclic value to JSON")
		}
		visiting[o] = true
		defer delete(visiting, o)
		out := make([]interface{}, len(v.Rows))
		for i, row := range v.Rows {
			rec := make(map[string]interface{}, len(v.Headers))
			for _, h := range v.Headers {
				conv, err := objectToJSONInner(rowCell(row, h), visiting)
				if err != nil {
					return nil, err
				}
				rec[h] = conv
			}
			out[i] = rec
		}
		return out, nil
	default:
		return nil, fmt.Errorf("cannot encode %s to JSON", o.Type())
	}
}
