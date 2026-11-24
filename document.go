package scl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// Document protects its source and effective configuration with a mutex.
// Snapshots and values returned to callers are detached copies.
type Document struct {
	mu sync.RWMutex
	path string
	source,original []byte
	schema Schema
	values object
}

func copySchema(s Schema)(Schema,error){
	if s==nil{return nil,nil}
	out:=Schema{}
	for k,f:=range s{
		f.Enum=append([]string(nil),f.Enum...)
		if f.Min!=nil{x:=*f.Min;f.Min=&x}
		if f.Max!=nil{x:=*f.Max;f.Max=&x}
		if f.Default!=nil{v,e:=normalize(f.Default);if e!=nil{return nil,e};f.Default=v}
		out[k]=f
	}
	return out,nil
}

func Parse(src []byte,filename string,schema Schema)(*Document,error){
	raw,err:=parseSource(src,filename);if err!=nil{return nil,err}
	schema,err=copySchema(schema);if err!=nil{return nil,err}
	values,err:=schema.apply(raw);if err!=nil{return nil,err}
	return &Document{source:bytes.Clone(src),original:bytes.Clone(src),schema:schema,values:values},nil
}

func readRegular(path string)([]byte,error){
	info,err:=os.Lstat(path);if err!=nil{return nil,err}
	if !info.Mode().IsRegular(){return nil,fmt.Errorf("not a regular file: %s",path)}
	f,err:=os.Open(path);if err!=nil{return nil,err};defer f.Close()
	b,err:=io.ReadAll(io.LimitReader(f,MaxBytes+1));if err!=nil{return nil,err}
	if len(b)>MaxBytes{return nil,fmt.Errorf("configuration exceeds size limit")}
	return b,nil
}

func Load(path string,schema Schema)(*Document,error){
	path,err:=filepath.Abs(path);if err!=nil{return nil,err}
	src,err:=readRegular(path);if err!=nil{return nil,err}
	d,err:=Parse(src,path,schema);if err!=nil{return nil,err}
	d.path=path;return d,nil
}
func (d *Document) Bytes()[]byte{d.mu.RLock();defer d.mu.RUnlock();return bytes.Clone(d.source)}
func (d *Document) Snapshot()map[string]any{d.mu.RLock();defer d.mu.RUnlock();return clone(d.values).(object)}
func (d *Document) Get(path string)(any,bool){d.mu.RLock();defer d.mu.RUnlock();v,ok:=lookup(d.values,path);return clone(v),ok}
func (d *Document) String(path string)string{v,_:=d.Get(path);s,_:=v.(string);return s}
func (d *Document) Bool(path string)bool{v,_:=d.Get(path);b,_:=v.(bool);return b}
func (d *Document) Int(path string)int64{v,_:=d.Get(path);n,_:=v.(json.Number);i,_:=n.Int64();return i}
func (d *Document) Duration(path string)time.Duration{v,_:=time.ParseDuration(d.String(path));return v}

func (d *Document) replace(src []byte)error{
	raw,err:=parseSource(src,d.path);if err!=nil{return err}
	values,err:=d.schema.apply(raw);if err!=nil{return err}
	d.source=bytes.Clone(src);d.values=values;return nil
}

// Set edits a root attribute or an attribute inside existing blocks. The
// candidate document is validated before replacing the current snapshot.
func (d *Document) Set(path string,value any)error{
	d.mu.Lock();defer d.mu.Unlock()
	parts:=strings.Split(path,".")
	for _,p:=range parts{if !validSegment(p){return fmt.Errorf("invalid path %q",path)}}
	f,diags:=hclwrite.ParseConfig(d.source,d.path,hcl.Pos{Line:1,Column:1})
	if diags.HasErrors(){return diags}
	body:=f.Body()
	for i:=0;i<len(parts)-1;{
		var found *hclwrite.Block
		consumed:=1
		for _,b:=range body.Blocks(){
			if b.Type()!=parts[i]{continue}
			l:=b.Labels()
			if len(l)==0{found=b;break}
			if len(l)==1&&i+1<len(parts)-1&&l[0]==parts[i+1]{found=b;consumed=2;break}
		}
		if found==nil{return fmt.Errorf("parent block not found: %s",strings.Join(parts[:i+1],"."))}
		body=found.Body();i+=consumed
	}
	key:=parts[len(parts)-1]
	if !hclsyntaxIdentifier(key){return fmt.Errorf("invalid attribute name %q",key)}
	v,err:=normalize(value);if err!=nil{return err}
	c,err:=toCty(v);if err!=nil{return err}
	body.SetAttributeValue(key,c)
	return d.replace(f.Bytes())
}

func (d *Document) Format()error{
	d.mu.Lock();defer d.mu.Unlock();return d.replace(hclwrite.Format(d.source))
}

// Reload publishes only a valid configuration; unsaved edits are not lost.
func (d *Document) Reload()error{
	d.mu.Lock();defer d.mu.Unlock()
	if d.path==""{return fmt.Errorf("document has no file")}
	if !bytes.Equal(d.source,d.original){return fmt.Errorf("unsaved changes")}
	src,err:=readRegular(d.path);if err!=nil{return err}
	if err=d.replace(src);err!=nil{return err}
	d.original=bytes.Clone(src);return nil
}

// Save rejects external changes detected before replacement. Callers must
// serialize writers across processes. Rename atomicity depends on the OS.
func (d *Document) Save()error{
	d.mu.Lock();defer d.mu.Unlock()
	if d.path==""{return fmt.Errorf("document has no file")}
	current,err:=readRegular(d.path);if err!=nil{return err}
	if !bytes.Equal(current,d.original){return fmt.Errorf("configuration changed on disk; reload first")}
	info,err:=os.Lstat(d.path);if err!=nil{return err}
	if !info.Mode().IsRegular(){return fmt.Errorf("not a regular file")}
	f,err:=os.CreateTemp(filepath.Dir(d.path),".scl-*");if err!=nil{return err}
	name:=f.Name();defer os.Remove(name);defer f.Close()
	if err=f.Chmod(info.Mode().Perm());err!=nil{return err}
	if _,err=f.Write(d.source);err!=nil{return err}
	if err=f.Sync();err!=nil{return err}
	if err=f.Close();err!=nil{return err}
	if err=os.Rename(name,d.path);err!=nil{return err}
	d.original=bytes.Clone(d.source);return nil
}
