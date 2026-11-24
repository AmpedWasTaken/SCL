package scl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ReadSchema loads an application-defined schema from a bounded JSON document.
// Schema files should be shipped by the application, not editable by tenants.
func ReadSchema(path string)(Schema,error){
	src,err:=readRegular(path)
	if err!=nil{return nil,err}
	d:=json.NewDecoder(bytes.NewReader(src))
	d.UseNumber()
	d.DisallowUnknownFields()
	var schema Schema
	if err=d.Decode(&schema);err!=nil{return nil,err}
	if schema==nil{return nil,fmt.Errorf("expected schema object")}
	var extra any
	if err=d.Decode(&extra);err!=io.EOF{return nil,fmt.Errorf("expected one schema document")}
	if err=schema.ValidateDefinition();err!=nil{return nil,err}
	return schema,nil
}

// ValidateDefinition checks field constraints and defaults, including those
// for optional settings which are absent from a configuration.
func (s Schema) ValidateDefinition()error{
	paths:=s.Paths()
	for _,path:=range paths{
		f:=s[path]
		if err:=validateFieldDefinition(path,f);err!=nil{return err}
		if len(f.Enum)>0 && f.Kind!=String{return fmt.Errorf("%s: enum requires string kind",path)}
		if f.Default!=nil{
			v,err:=normalize(f.Default)
			if err!=nil{return fmt.Errorf("%s: invalid default",path)}
			if err=checkField(path,f,v);err!=nil{return err}
		}
		parts:=strings.Split(path,".")
		for i:=1;i<len(parts);i++{
			if ancestor,ok:=s[strings.Join(parts[:i],".")];ok && ancestor.Kind!=Object{
				return fmt.Errorf("%s: parent schema field is not an object",path)
			}
		}
	}
	return nil
}

// Generate emits a starter configuration containing non-sensitive defaults.
// Required fields without a default must be filled by the operator; this
// template is not necessarily a complete valid configuration.
func (s Schema) Generate()([]byte,error){
	if err:=s.ValidateDefinition();err!=nil{return nil,err}
	o:=object{}
	var hints strings.Builder
	hints.WriteString("# Generated SCL configuration. Fill required settings before use.\n")
	for _,path:=range s.Paths(){
		f:=s[path]
		if f.Sensitive||f.Kind==Secret||f.Default==nil||f.Kind==Object{
			fmt.Fprintf(&hints,"# %s (%s; required=%t): set explicitly if needed\n",path,f.Kind,f.Required)
			continue
		}
		v,err:=normalize(f.Default)
		if err!=nil{return nil,err}
		if err=put(o,path,v);err!=nil{return nil,err}
	}
	raw,err:=json.Marshal(o)
	if err!=nil{return nil,err}
	out,err:=FromJSON(raw)
	if err!=nil{return nil,err}
	return append([]byte(hints.String()+"\n"),out...),nil
}

// Describe returns deterministic JSON metadata, intentionally omitting all
// default values and enum values, so secret values are not exported.
func (s Schema) Describe()([]byte,error){
	if err:=s.ValidateDefinition();err!=nil{return nil,err}
	type fieldInfo struct{
		Path string
		Kind Kind
		Description string
		Required,HasDefault,HotReload,Sensitive bool
		Element Kind
		MinLength,MaxLength int
	}
	entries:=make([]fieldInfo,0,len(s))
	for _,p:=range s.Paths(){
		f:=s[p]
		entries=append(entries,fieldInfo{Path:p,Kind:f.Kind,Description:f.Description,Required:f.Required,HasDefault:f.Default!=nil,HotReload:f.HotReload,Sensitive:f.Sensitive||f.Kind==Secret,Element:f.Element,MinLength:f.MinLength,MaxLength:f.MaxLength})
	}
	return json.MarshalIndent(entries,"","  ")
}

// Decode converts the effective snapshot into a Go struct using JSON tags.
// The destination receives references, not resolved secrets. Unknown fields
// are rejected to catch mismatches between Go types and the configuration.
func (d *Document) Decode(dest any)error{
	raw,err:=json.Marshal(d.Snapshot())
	if err!=nil{return err}
	dec:=json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	return dec.Decode(dest)
}

// ValidateWith allows application-defined cross-field checks. The callback
// receives a detached snapshot and must not perform external side effects.
func (d *Document) ValidateWith(check func(map[string]any)error)error{
	if check==nil{return fmt.Errorf("validation callback required")}
	return check(d.Snapshot())
}

// SchemaKeys returns sorted field names for editor or tooling integrations.
func SchemaKeys(s Schema)[]string{
	keys:=s.Paths()
	sort.Strings(keys)
	return keys
}
