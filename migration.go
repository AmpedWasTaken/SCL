package scl

import (
	"fmt"
	"sort"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

func hclsyntaxIdentifier(s string)bool{return hclsyntax.ValidIdentifier(s)}

// FromJSON converts a JSON object to literal SCL. It does not write files.
func FromJSON(src []byte)([]byte,error){
	if len(src)>MaxBytes{return nil,fmt.Errorf("JSON exceeds size limit")}
	v,err:=decodeJSON(src);if err!=nil{return nil,err}
	o,ok:=v.(object);if !ok{return nil,fmt.Errorf("JSON root must be an object")}
	file:=hclwrite.NewEmptyFile()
	var write func(*hclwrite.Body,object,int)error
	write=func(b *hclwrite.Body,o object,depth int)error{
		if depth>MaxDepth{return fmt.Errorf("nesting limit exceeded")}
		keys:=make([]string,0,len(o));for k:=range o{keys=append(keys,k)};sort.Strings(keys)
		for _,k:=range keys{
			if !hclsyntaxIdentifier(k)||k=="use"||k=="preset"{return fmt.Errorf("reserved or invalid key %q",k)}
			if nested,ok:=o[k].(object);ok{
				block:=b.AppendNewBlock(k,nil)
				if err:=write(block.Body(),nested,depth+1);err!=nil{return err}
			}else{
				v,err:=toCty(o[k]);if err!=nil{return err};b.SetAttributeValue(k,v)
			}
		}
		return nil
	}
	if err=write(file.Body(),o,0);err!=nil{return nil,err}
	out:=file.Bytes()
	if _,err=Parse(out,"migration.conf",nil);err!=nil{return nil,err}
	return out,nil
}
