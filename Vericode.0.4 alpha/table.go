package main

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// ─── Table (0.4) ─────────────────────────────────────────────────────────────
//
// A Table is rows of hashes plus an ordered header list. Rows are real Hash
// objects, so t[0]["age"] = 40 mutates the table in place, just like arrays
// and hashes elsewhere in the language. Cell values are shared by reference.

type Table struct {
	Headers []string
	Rows    []*Hash
}

func (t *Table) Type() ObjectType { return TABLE_OBJ }

func (t *Table) Inspect() string {
	if len(t.Headers) == 0 {
		return "(empty table)"
	}
	widths := make([]int, len(t.Headers))
	for i, h := range t.Headers {
		widths[i] = utf8.RuneCountInString(h)
	}
	cells := make([][]string, len(t.Rows))
	for r, row := range t.Rows {
		cells[r] = make([]string, len(t.Headers))
		for c, h := range t.Headers {
			cell := rowCell(row, h)
			s := cellString(cell)
			cells[r][c] = s
			if n := utf8.RuneCountInString(s); n > widths[c] {
				widths[c] = n
			}
		}
	}
	var b strings.Builder
	writeLine := func(vals []string) {
		parts := make([]string, len(vals))
		for i, v := range vals {
			parts[i] = padRight(v, widths[i])
		}
		b.WriteString(strings.TrimRight(strings.Join(parts, "  "), " "))
		b.WriteByte('\n')
	}
	writeLine(t.Headers)
	sep := make([]string, len(t.Headers))
	for i, w := range widths {
		sep[i] = strings.Repeat("-", w)
	}
	writeLine(sep)
	for _, row := range cells {
		writeLine(row)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func padRight(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

func cellString(o Object) string {
	if o == nil {
		return "null"
	}
	return o.Inspect()
}

// cellKey maps a column name to the string key used by row hashes.
func cellKey(name string) string { return (&String{Value: name}).HashKey() }

func rowCell(row *Hash, col string) Object {
	if row == nil {
		return &Null{}
	}
	if pair, ok := row.Pairs[cellKey(col)]; ok && pair.Value != nil {
		return pair.Value
	}
	return &Null{}
}

func (t *Table) columnIndex(name string) int {
	for i, h := range t.Headers {
		if h == name {
			return i
		}
	}
	return -1
}

func (t *Table) column(name string) []Object {
	out := make([]Object, len(t.Rows))
	for i, row := range t.Rows {
		out[i] = rowCell(row, name)
	}
	return out
}

func copyRowHash(row *Hash) *Hash {
	pairs := make(map[string]HashPairObj, len(row.Pairs))
	for k, pair := range row.Pairs {
		pairs[k] = pair
	}
	return &Hash{Pairs: pairs}
}

// tableFromHashes builds a table from an array of hash objects. Without
// explicit headers the union of keys is used in sorted order, so construction
// is deterministic even though Go maps are not ordered.
func tableFromHashes(op string, rows []Object, headers []string, headersGiven bool) Object {
	for i, r := range rows {
		if _, ok := r.(*Hash); !ok {
			return newError("%s() row %d must be a hash, got %s", op, i, r.Type())
		}
	}

	if headersGiven {
		seen := make(map[string]bool, len(headers))
		for _, h := range headers {
			if seen[h] {
				return newError("%s() duplicate column '%s'", op, h)
			}
			seen[h] = true
		}
		for i, r := range rows {
			for _, pair := range r.(*Hash).Pairs {
				name := pair.Key.Inspect()
				if !seen[name] {
					return newError("%s() row %d has column '%s' not listed in headers", op, i, name)
				}
			}
		}
	} else {
		keySet := make(map[string]bool)
		for _, r := range rows {
			for _, pair := range r.(*Hash).Pairs {
				keySet[pair.Key.Inspect()] = true
			}
		}
		headers = make([]string, 0, len(keySet))
		for k := range keySet {
			headers = append(headers, k)
		}
		sort.Strings(headers)
	}

	t := &Table{
		Headers: append([]string(nil), headers...),
		Rows:    make([]*Hash, len(rows)),
	}
	for i, r := range rows {
		pairs := make(map[string]HashPairObj, len(headers))
		for _, col := range headers {
			pairs[cellKey(col)] = HashPairObj{Key: &String{Value: col}, Value: rowCell(r.(*Hash), col)}
		}
		t.Rows[i] = &Hash{Pairs: pairs}
	}
	return t
}

// ─── Builtins ────────────────────────────────────────────────────────────────

func tableBuiltins() []*Builtin {
	return []*Builtin{
		{
			Name: "table",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) < 1 || len(args) > 2 {
					return newError("table() takes 1-2 arguments (array of hashes, [headers]), got %d", len(args))
				}
				rows, ok := args[0].(*Array)
				if !ok {
					return newError("table() first argument must be an array of hashes, got %s", args[0].Type())
				}
				var headers []string
				headersGiven := false
				if len(args) == 2 {
					harr, ok := args[1].(*Array)
					if !ok {
						return newError("table() second argument must be an array of column names, got %s", args[1].Type())
					}
					headers = make([]string, len(harr.Elements))
					for i, el := range harr.Elements {
						s, ok := el.(*String)
						if !ok {
							return newError("table() header %d must be a string, got %s", i, el.Type())
						}
						headers[i] = s.Value
					}
					headersGiven = true
				}
				return tableFromHashes("table", rows.Elements, headers, headersGiven)
			},
		},
		{
			Name: "columns",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("columns() takes 1 argument (table), got %d", len(args))
				}
				t, ok := args[0].(*Table)
				if !ok {
					return newError("columns() argument must be a table, got %s", args[0].Type())
				}
				out := make([]Object, len(t.Headers))
				for i, h := range t.Headers {
					out[i] = &String{Value: h}
				}
				return &Array{Elements: out}
			},
		},
		{
			Name: "rows",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("rows() takes 1 argument (table), got %d", len(args))
				}
				t, ok := args[0].(*Table)
				if !ok {
					return newError("rows() argument must be a table, got %s", args[0].Type())
				}
				out := make([]Object, len(t.Rows))
				for i, r := range t.Rows {
					out[i] = r
				}
				return &Array{Elements: out}
			},
		},
		{
			Name: "tselect",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) < 2 {
					return newError("tselect() takes at least 2 arguments (table, columns...), got %d", len(args))
				}
				t, ok := args[0].(*Table)
				if !ok {
					return newError("tselect() first argument must be a table, got %s", args[0].Type())
				}
				cols := make([]string, len(args)-1)
				seen := make(map[string]bool, len(cols))
				for i, arg := range args[1:] {
					s, ok := arg.(*String)
					if !ok {
						return newError("tselect() column %d must be a string, got %s", i, arg.Type())
					}
					if t.columnIndex(s.Value) < 0 {
						return newError("tselect() unknown column '%s'; columns are %s", s.Value, strings.Join(t.Headers, ", "))
					}
					if seen[s.Value] {
						return newError("tselect() duplicate column '%s'", s.Value)
					}
					seen[s.Value] = true
					cols[i] = s.Value
				}
				out := &Table{Headers: cols, Rows: make([]*Hash, len(t.Rows))}
				for i, row := range t.Rows {
					pairs := make(map[string]HashPairObj, len(cols))
					for _, col := range cols {
						pairs[cellKey(col)] = HashPairObj{Key: &String{Value: col}, Value: rowCell(row, col)}
					}
					out.Rows[i] = &Hash{Pairs: pairs}
				}
				return out
			},
		},
		{
			Name: "twhere",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 2 {
					return newError("twhere() takes 2 arguments (table, func), got %d", len(args))
				}
				t, ok := args[0].(*Table)
				if !ok {
					return newError("twhere() first argument must be a table, got %s", args[0].Type())
				}
				if !isFunc(args[1]) {
					return newError("twhere() second argument must be a function, got %s", args[1].Type())
				}
				out := &Table{Headers: append([]string(nil), t.Headers...), Rows: []*Hash{}}
				for _, row := range t.Rows {
					keep := callUserFn(vm, args[1], row)
					if e, ok := keep.(*Error); ok {
						return e
					}
					if isTruthy(keep) {
						out.Rows = append(out.Rows, copyRowHash(row))
					}
				}
				return out
			},
		},
		{
			Name: "tsort",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) < 2 || len(args) > 3 {
					return newError("tsort() takes 2-3 arguments (table, column, [asc|desc]), got %d", len(args))
				}
				t, ok := args[0].(*Table)
				if !ok {
					return newError("tsort() first argument must be a table, got %s", args[0].Type())
				}
				col, ok := args[1].(*String)
				if !ok {
					return newError("tsort() column must be a string, got %s", args[1].Type())
				}
				if t.columnIndex(col.Value) < 0 {
					return newError("tsort() unknown column '%s'; columns are %s", col.Value, strings.Join(t.Headers, ", "))
				}
				desc := false
				if len(args) == 3 {
					order, ok := args[2].(*String)
					if !ok {
						return newError("tsort() order must be \"asc\" or \"desc\", got %s", args[2].Type())
					}
					switch strings.ToLower(order.Value) {
					case "asc":
					case "desc":
						desc = true
					default:
						return newError("tsort() order must be \"asc\" or \"desc\", got '%s'", order.Value)
					}
				}
				rows := make([]*Hash, len(t.Rows))
				for i, r := range t.Rows {
					rows[i] = copyRowHash(r)
				}
				var cmpErr error
				sort.SliceStable(rows, func(i, j int) bool {
					if cmpErr != nil {
						return false
					}
					av := rowCell(rows[i], col.Value)
					bv := rowCell(rows[j], col.Value)
					aNull := av.Type() == NULL_OBJ
					bNull := bv.Type() == NULL_OBJ
					if aNull || bNull {
						// Nulls stay last in both directions.
						if aNull && bNull {
							return false
						}
						return bNull && !aNull
					}
					c, err := compareValues(av, bv)
					if err != nil {
						cmpErr = err
						return false
					}
					if desc {
						return c > 0
					}
					return c < 0
				})
				if cmpErr != nil {
					return newError("tsort() column '%s': %v", col.Value, cmpErr)
				}
				return &Table{Headers: append([]string(nil), t.Headers...), Rows: rows}
			},
		},
		{
			Name: "tgroup",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 2 {
					return newError("tgroup() takes 2 arguments (table, column), got %d", len(args))
				}
				t, ok := args[0].(*Table)
				if !ok {
					return newError("tgroup() first argument must be a table, got %s", args[0].Type())
				}
				col, ok := args[1].(*String)
				if !ok {
					return newError("tgroup() column must be a string, got %s", args[1].Type())
				}
				if t.columnIndex(col.Value) < 0 {
					return newError("tgroup() unknown column '%s'; columns are %s", col.Value, strings.Join(t.Headers, ", "))
				}
				groups := make(map[string]HashPairObj)
				for _, row := range t.Rows {
					val := rowCell(row, col.Value)
					hashable, ok := val.(Hashable)
					if !ok {
						return newError("tgroup() values in column '%s' must be hashable (integer, float, boolean, or string), got %s", col.Value, val.Type())
					}
					pair, exists := groups[hashable.HashKey()]
					var group *Table
					if exists {
						group = pair.Value.(*Table)
					} else {
						group = &Table{Headers: append([]string(nil), t.Headers...), Rows: []*Hash{}}
						groups[hashable.HashKey()] = HashPairObj{Key: val, Value: group}
					}
					group.Rows = append(group.Rows, copyRowHash(row))
				}
				return &Hash{Pairs: groups}
			},
		},
	}
}
