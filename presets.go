package scl

import (
	"fmt"
	"sort"
)

// use references a named root preset. Explicit values win; nested objects
// merge recursively, and arrays replace rather than append.
func resolvePresets(raw object) (object,error) {
	presets := object{}
	if v,ok := raw["preset"]; ok {
		var valid bool
		presets,valid = v.(object)
		if !valid { return nil,fmt.Errorf("preset must be a named block") }
		delete(raw,"preset")
	}
	// Bound cumulative copying, not just source size; repeated references to
	// nested presets can otherwise expand exponentially.
	budget := 8 * MaxBytes
	var spend func(any,int) error
	spend = func(v any, depth int) error {
		if depth > MaxDepth { return fmt.Errorf("expanded preset nesting exceeds limit") }
		budget -= 32
		switch x := v.(type) {
		case object:
			for k,item := range x { budget -= len(k); if err:=spend(item,depth+1);err!=nil{return err} }
		case []any:
			for _,item := range x { if err:=spend(item,depth+1);err!=nil{return err} }
		case string: budget -= len(x)
		}
		if budget < 0 { return fmt.Errorf("expanded configuration exceeds processing budget") }
		return nil
	}
	cache := map[string]object{}
	active := map[string]bool{}
	var resolve func(object,int)(object,error)
	var preset func(string,int)(object,error)
	preset = func(name string,depth int)(object,error) {
		if depth>MaxDepth || active[name] { return nil,fmt.Errorf("cyclic or too-deep preset %q",name) }
		if v,ok := cache[name]; ok { if err:=spend(v,0);err!=nil{return nil,err};return clone(v).(object),nil }
		v,ok := presets[name].(object); if !ok { return nil,fmt.Errorf("unknown or invalid preset %q",name) }
		active[name]=true
		result,err := resolve(v,depth+1)
		delete(active,name)
		if err!=nil { return nil,err }
		if err:=spend(result,0);err!=nil{return nil,err}
		cache[name]=result
		return clone(result).(object),nil
	}
	resolve = func(src object,depth int)(object,error) {
		if depth>MaxDepth { return nil,fmt.Errorf("preset nesting limit exceeded") }
		out := object{}
		if use,ok := src["use"]; ok {
			name,ok := use.(string); if !ok { return nil,fmt.Errorf("use must be a preset name string") }
			base,err := preset(name,depth+1); if err!=nil { return nil,err }
			out=base
		}
		keys:=make([]string,0,len(src)); for k:=range src { keys=append(keys,k) }; sort.Strings(keys)
		for _,k:=range keys {
			if k=="use" { continue }
			v:=src[k]
			if nested,ok:=v.(object); ok {
				next,err:=resolve(nested,depth+1); if err!=nil{return nil,err}
				if base,ok:=out[k].(object);ok { if err:=spend(base,0);err!=nil{return nil,err};if err:=spend(next,0);err!=nil{return nil,err};v=merge(base,next) } else {v=next}
			}
			if err:=spend(v,0);err!=nil{return nil,err}
			out[k]=clone(v)
		}
		if err:=spend(out,0);err!=nil{return nil,err}
		return out,nil
	}
	for name:=range presets { if _,err:=preset(name,0);err!=nil{return nil,err} }
	return resolve(raw,0)
}

func merge(base,override object)object {
	out:=clone(base).(object)
	for k,v:=range override {
		if a,ok:=out[k].(object);ok {
			if b,ok:=v.(object);ok { out[k]=merge(a,b);continue }
		}
		out[k]=clone(v)
	}
	return out
}
