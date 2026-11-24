package scl

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ProjectOptions are application-selected, not read from configuration.
// Root enables imports and is the boundary for every file. Profile selects
// one optional named overlay; an empty value activates only the base.
type ProjectOptions struct {
	Root string
	Profile string
}

const MaxProjectFiles = 32
const MaxProjectBytes = 4 * MaxBytes

// LoadProject reads a composed configuration and returns detached effective
// values. Editing individual project sources is intentionally a separate task.
func LoadProject(entry string, schema Schema, opts ProjectOptions) (map[string]any, error) {
	if opts.Root == "" { return nil, fmt.Errorf("project root is required") }
	root,err:=filepath.Abs(opts.Root)
	if err!=nil{return nil,err}
	root,err=filepath.EvalSymlinks(root)
	if err!=nil{return nil,err}
	info,err:=os.Stat(root)
	if err!=nil||!info.IsDir(){return nil,fmt.Errorf("invalid project root")}
	path,err:=filepath.Abs(entry)
	if err!=nil{return nil,err}
	// Resolve the entry's parent alias so a symlinked root supplied by the
	// operator works, without permitting import symlinks inside the root.
	absoluteRoot,err:=filepath.Abs(opts.Root)
	if err!=nil{return nil,err}
	rel,err:=filepath.Rel(absoluteRoot,path)
	if err!=nil{return nil,err}
	name:=filepath.ToSlash(rel)
	active,seen:=map[string]bool{},map[string]bool{}
	total:=0
	var read func(string,int)(object,error)
	read=func(name string,depth int)(object,error){
		if depth>MaxDepth{return nil,fmt.Errorf("import nesting exceeds limit")}
		if !fs.ValidPath(name)||name=="."||strings.ContainsAny(name,"\\:\x00")||!supportedProjectExtension(name){
			return nil,fmt.Errorf("invalid import path %q",name)
		}
		path:=filepath.Join(root,filepath.FromSlash(name))
		// Reject symlinks at every path component, including the final file.
		// The application must prevent concurrent untrusted directory changes.
		current:=root
		for _,part:=range strings.Split(name,"/"){
			current=filepath.Join(current,part)
			info,err:=os.Lstat(current)
			if err!=nil{return nil,fmt.Errorf("import %s: %w",name,err)}
			if info.Mode()&os.ModeSymlink!=0{return nil,fmt.Errorf("symlink import rejected: %s",name)}
		}
		real,err:=filepath.EvalSymlinks(path)
		if err!=nil{return nil,err}
		relative,err:=filepath.Rel(root,real)
		if err!=nil||relative==".."||strings.HasPrefix(relative,".."+string(filepath.Separator)){return nil,fmt.Errorf("import outside project root")}
		identity:=real
		if filepath.Separator=='\\'{identity=strings.ToLower(identity)}
		if active[identity]{return nil,fmt.Errorf("cyclic import: %s",name)}
		if seen[identity]{return nil,fmt.Errorf("duplicate import: %s",name)}
		if len(seen)>=MaxProjectFiles{return nil,fmt.Errorf("too many project files")}
		active[identity],seen[identity]=true,true
		defer delete(active,identity)
		src,err:=readRegular(path)
		if err!=nil{return nil,err}
		total+=len(src)
		if total>MaxProjectBytes{return nil,fmt.Errorf("project exceeds size limit")}
		raw,err:=parseRawSource(src,name)
		if err!=nil{return nil,err}
		out:=object{}
		if imports,ok:=raw["imports"];ok{
			items,ok:=imports.([]any)
			if !ok{return nil,fmt.Errorf("%s: imports must be an array of paths",name)}
			delete(raw,"imports")
			for _,item:=range items{
				child,ok:=item.(string)
				if !ok{return nil,fmt.Errorf("%s: import path must be a string",name)}
				next,err:=read(child,depth+1)
				if err!=nil{return nil,err}
				if err=mergeDistinct(out,next,"");err!=nil{return nil,fmt.Errorf("%s: %w",name,err)}
			}
		}
		if err=mergeDistinct(out,raw,"");err!=nil{return nil,fmt.Errorf("%s: %w",name,err)}
		return out,nil
	}
	raw,err:=read(name,0)
	if err!=nil{return nil,err}
	profiles:=object{}
	if v,ok:=raw["profile"];ok{
		var valid bool
		profiles,valid=v.(object)
		if !valid{return nil,fmt.Errorf("profile must contain named overlays")}
		delete(raw,"profile")
	}
	for _,name:=range sortedObjectKeys(profiles){
		p,ok:=profiles[name].(object)
		if !ok{return nil,fmt.Errorf("invalid profile %q",name)}
		for _,reserved:=range []string{"imports","profile","preset"}{
			if _,ok:=p[reserved];ok{return nil,fmt.Errorf("profile %q contains reserved key %q",name,reserved)}
		}
	}
	if opts.Profile!=""{
		selected,ok:=profiles[opts.Profile].(object)
		if !ok{return nil,fmt.Errorf("unknown profile %q",opts.Profile)}
		raw=merge(raw,selected)
	}
	effective,err:=resolvePresets(raw)
	if err!=nil{return nil,err}
	return schema.apply(effective)
}

func supportedProjectExtension(name string)bool{
	switch strings.ToLower(filepath.Ext(name)){case ".scl",".conf":return true}
	return false
}

func sortedObjectKeys(o object)[]string{
	keys:=make([]string,0,len(o))
	for k:=range o{keys=append(keys,k)}
	sort.Strings(keys)
	return keys
}

// Merge disjoint branches in place. A duplicate leaf is always an error,
// even if both values happen to be equal.
func mergeDistinct(dst,src object,prefix string)error{
	for _,k:=range sortedObjectKeys(src){
		v:=src[k]
		if old,ok:=dst[k];ok{
			a,aok:=old.(object);b,bok:=v.(object)
			if !aok||!bok{return fmt.Errorf("duplicate imported setting %s%s",prefix,k)}
			if err:=mergeDistinct(a,b,prefix+k+".");err!=nil{return err}
			continue
		}
		dst[k]=v
	}
	return nil
}
