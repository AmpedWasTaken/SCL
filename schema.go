package scl

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Kind string
const (
	String Kind = "string"
	Integer Kind = "integer"
	Number Kind = "number"
	Boolean Kind = "boolean"
	Array Kind = "array"
	Object Kind = "object"
	Duration Kind = "duration"
	IP Kind = "ip"
	CIDR Kind = "cidr"
	Port Kind = "port"
	Hostname Kind = "hostname"
	URL Kind = "url"
	Email Kind = "email"
	Color Kind = "color"
	Percentage Kind = "percentage"
	Bytes Kind = "bytes"
)

// Field describes a supported setting. Nil Default means no default.
// Integer bounds are inclusive. Enum applies to string values.
type Field struct {
	Kind Kind
	Description string
	Default any
	Required bool
	Min,Max *int64
	Enum []string
	HotReload bool
	Element Kind // optional homogeneous array element type
	Pattern string // RE2 pattern for string values
	MinLength,MaxLength int // zero means unspecified; Unicode code points or collection items
	Sensitive bool // metadata for documentation and host-side display policies
}

type Schema map[string]Field

func (s Schema) Paths() []string {
	keys:=make([]string,0,len(s));for k:=range s {keys=append(keys,k)};sort.Strings(keys);return keys
}

func put(root object,path string,v any) error {
	parts:=strings.Split(path,".")
	for _,k:=range parts {if !validSegment(k){return fmt.Errorf("invalid schema path %q",path)}}
	for _,k:=range parts[:len(parts)-1] {
		if _,ok:=root[k];!ok{root[k]=object{}}
		next,ok:=root[k].(object);if !ok{return fmt.Errorf("%s: parent is not an object",path)}
		root=next
	}
	root[parts[len(parts)-1]]=v
	return nil
}

func checkField(path string,f Field,v any)error {
	ok:=false
	switch f.Kind {
	case String: _,ok=v.(string)
	case Boolean: _,ok=v.(bool)
	case Number: _,ok=v.(json.Number)
	case Integer:
		if n,is:=v.(json.Number);is {
			i,e:=n.Int64();ok=e==nil
			if ok && ((f.Min!=nil && i<*f.Min)||(f.Max!=nil && i>*f.Max)){return fmt.Errorf("%s: integer outside permitted bounds",path)}
		}
	case Array: _,ok=v.([]any)
	case Object: _,ok=v.(object)
	case Duration:
		if st,is:=v.(string);is{_,e:=time.ParseDuration(st);ok=e==nil}
	case IP,CIDR,Port,Hostname,URL,Email,Color,Percentage,Bytes: ok=checkExtendedKind(f.Kind,v)
	default:return fmt.Errorf("%s: invalid schema kind %q",path,f.Kind)
	}
	if !ok{return fmt.Errorf("%s: expected %s, got %T",path,f.Kind,v)}
	if len(f.Enum)>0{
		str,is:=v.(string);match:=false
		for _,choice:=range f.Enum {if is && str==choice{match=true}}
		if !match{return fmt.Errorf("%s: expected one of %s",path,strings.Join(f.Enum,", "))}
	}
	return checkConstraints(path,f,v)
}

// apply validates a strict schema and returns a detached effective configuration.
// A nil schema permits arbitrary literal keys.
func (s Schema) apply(raw object)(object,error){
	out:=clone(raw).(object)
	if s==nil{return out,nil}
	for _,p:=range s.Paths(){
		f:=s[p]
		if err:=validateFieldDefinition(p,f);err!=nil{return nil,err}
		v,ok:=lookup(out,p)
		if !ok && f.Default!=nil{
			var err error
			v,err=normalize(f.Default);if err!=nil{return nil,fmt.Errorf("%s: %w",p,err)}
			if err=put(out,p,v);err!=nil{return nil,err};ok=true
		}
		if !ok {if f.Required{return nil,fmt.Errorf("%s: required setting missing",p)};continue}
		if err:=checkField(p,f,v);err!=nil{return nil,err}
	}
	var walk func(object,string)error
	walk=func(o object,prefix string)error{
		keys:=make([]string,0,len(o));for k:=range o{keys=append(keys,k)};sort.Strings(keys)
		for _,k:=range keys{
			if !validSegment(k){return fmt.Errorf("invalid setting name %q",k)}
			p:=prefix+k
			if f,ok:=s[p];ok&&f.Kind==Object{continue}
			if _,ok:=s[p];ok{continue}
			known:=false
			for path:=range s{if strings.HasPrefix(path,p+"."){known=true;break}}
			nested,is:=o[k].(object)
			if !known||!is{return fmt.Errorf("%s: unknown setting",p)}
			if err:=walk(nested,p+".");err!=nil{return err}
		}
		return nil
	}
	if err:=walk(out,"");err!=nil{return nil,err}
	return out,nil
}
