// Package scl implements declarative, schema-validated configuration for Go.
package scl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

const MaxBytes = 1 << 20
const MaxDepth = 32

type object = map[string]any

func decodeJSON(raw []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil { return nil, err }
	var extra any
	if err := d.Decode(&extra); err != io.EOF { return nil, fmt.Errorf("expected exactly one JSON value") }
	return v, nil
}

// Only literal expressions are accepted. No variables, functions, interpolation,
// comprehensions, traversals, arithmetic, or external resources.
func literal(e hclsyntax.Expression, depth int) bool {
	if depth > MaxDepth { return false }
	switch x := e.(type) {
	case *hclsyntax.LiteralValueExpr:
		return true
	case *hclsyntax.TemplateExpr:
		return x.IsStringLiteral()
	case *hclsyntax.UnaryOpExpr:
		if x.Op != hclsyntax.OpNegate { return false }
		v, ok := x.Val.(*hclsyntax.LiteralValueExpr)
		return ok && v.Val.Type() == cty.Number
	case *hclsyntax.TupleConsExpr:
		for _, v := range x.Exprs { if !literal(v, depth+1) { return false } }
		return true
	case *hclsyntax.ObjectConsExpr:
		for _, item := range x.Items {
			if len(item.KeyExpr.Variables()) != 0 || !literal(item.ValueExpr, depth+1) { return false }
			// Object keys are HCL's literal identifiers or quoted strings.
			k, diags := item.KeyExpr.Value(nil)
			if diags.HasErrors() || !k.IsKnown() || k.IsNull() || k.Type() != cty.String { return false }
		}
		return true
	default:
		return false
	}
}

func readBody(b *hclsyntax.Body, depth int) (object, error) {
	if depth > MaxDepth { return nil, fmt.Errorf("configuration nesting exceeds %d", MaxDepth) }
	out := object{}
	named := map[string]bool{}
	keys := make([]string, 0, len(b.Attributes))
	for k := range b.Attributes { keys = append(keys, k) }
	sort.Strings(keys)
	for _, k := range keys {
		a := b.Attributes[k]
		if !literal(a.Expr, depth+1) { return nil, fmt.Errorf("%s: only literal values are supported", a.Range().String()) }
		val, diags := a.Expr.Value(nil)
		if diags.HasErrors() { return nil, diags }
		raw, err := ctyjson.Marshal(val, val.Type())
		if err != nil { return nil, err }
		v, err := decodeJSON(raw)
		if err != nil { return nil, err }
		out[k] = v
	}
	for _, block := range b.Blocks {
		v, err := readBody(block.Body, depth+1)
		if err != nil { return nil, err }
		switch len(block.Labels) {
		case 0:
			if _, exists := out[block.Type]; exists { return nil, fmt.Errorf("%s: duplicate block or attribute %q", block.TypeRange.String(), block.Type) }
			out[block.Type] = v
		case 1:
			name := block.Labels[0]
			if !validSegment(name) { return nil, fmt.Errorf("%s: invalid block label %q", block.TypeRange.String(), name) }
			if _, exists := out[block.Type]; exists && !named[block.Type] { return nil, fmt.Errorf("%s: conflicting named block", block.TypeRange.String()) }
			if !named[block.Type] { out[block.Type] = object{}; named[block.Type] = true }
			group := out[block.Type].(object)
			if _, exists := group[name]; exists { return nil, fmt.Errorf("%s: duplicate named block %q", block.TypeRange.String(), name) }
			group[name] = v
		default:
			return nil, fmt.Errorf("%s: at most one block label is supported", block.TypeRange.String())
		}
	}
	return out, nil
}

func parseSource(src []byte, filename string) (object, error) {
	if len(src) > MaxBytes { return nil, fmt.Errorf("configuration exceeds %d bytes", MaxBytes) }
	file, diags := hclsyntax.ParseConfig(src, filename, hcl.Pos{Line:1, Column:1})
	if diags.HasErrors() { return nil, diags }
	raw, err := readBody(file.Body.(*hclsyntax.Body), 0)
	if err != nil { return nil, err }
	return resolvePresets(raw)
}

func validSegment(s string) bool {
	return s != "" && !strings.Contains(s, ".") && !strings.ContainsAny(s, "\x00\r\n")
}

func clone(v any) any {
	switch x := v.(type) {
	case object:
		o := object{}
		for k, v := range x { o[k] = clone(v) }
		return o
	case []any:
		a := make([]any, len(x)); for i, v := range x { a[i] = clone(v) }; return a
	default: return v
	}
}

func lookup(root object, path string) (any, bool) {
	var v any = root
	for _, k := range strings.Split(path, ".") {
		o, ok := v.(object); if !ok { return nil, false }
		v, ok = o[k]; if !ok { return nil, false }
	}
	return v, true
}

func toCty(v any) (cty.Value, error) {
	switch x := v.(type) {
	case nil: return cty.NullVal(cty.DynamicPseudoType), nil
	case bool: return cty.BoolVal(x), nil
	case string: return cty.StringVal(x), nil
	case json.Number: return cty.ParseNumberVal(string(x))
	case []any:
		vals := make([]cty.Value, 0, len(x))
		for _, item := range x { v, err := toCty(item); if err != nil { return cty.NilVal, err }; vals = append(vals,v) }
		return cty.TupleVal(vals), nil
	case object:
		vals := map[string]cty.Value{}
		for k,item := range x { v, err := toCty(item); if err != nil { return cty.NilVal, err }; vals[k]=v }
		return cty.ObjectVal(vals), nil
	default: return cty.NilVal, fmt.Errorf("unsupported configuration value %T",v)
	}
}

func normalize(v any) (any,error) {
	b,err := json.Marshal(v); if err != nil { return nil,err }
	if len(b)>MaxBytes { return nil,fmt.Errorf("value exceeds size limit") }
	return decodeJSON(b)
}
