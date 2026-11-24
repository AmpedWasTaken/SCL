package scl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectProfilesAndReload(t *testing.T){
	root:=t.TempDir()
	entry:=filepath.Join(root,"server.scl")
	part:=filepath.Join(root,"theme.conf")
	write:=func(path,s string){t.Helper();if err:=os.WriteFile(path,[]byte(s),0600);err!=nil{t.Fatal(err)}}
	write(entry,"imports = [\"theme.conf\"]\nserver { name = \"demo\" }\nprofile production {\n theme { fps = 20 }\n}\n")
	write(part,"preset animated { fps = 30 }\ntheme {\n use = \"animated\"\n color = \"blue\"\n}\n")
	s:=Schema{"server.name":{Kind:String},"theme.fps":{Kind:Integer,HotReload:true},"theme.color":{Kind:String,HotReload:true}}
	base,err:=LoadProject(entry,s,ProjectOptions{Root:root})
	if err!=nil{t.Fatal(err)}
	if n,_:=lookup(base,"theme.fps");n==nil||n.(interface{String()string}).String()!="30"{t.Fatal("base preset missing")}
	opts:=ProjectOptions{Root:root,Profile:"production"}
	m,err:=OpenManager([]FileSpec{{Name:"server",Path:entry,Schema:s,Project:opts}})
	if err!=nil{t.Fatal(err)}
	if n,_:=m.Get("server","theme.fps");n==nil||n.(interface{String()string}).String()!="20"{t.Fatal("profile missing")}
	write(part,"theme { color = \"green\" }\n")
	r,err:=m.Reload()
	if err!=nil||r.Generation!=2{t.Fatalf("import reload: %+v %v",r,err)}
	if v,_:=m.Get("server","theme.color");v!="green"{t.Fatal("import update missing")}
	write(part,"theme { color = 42 }\n")
	if _,err=m.Reload();err==nil{t.Fatal("invalid import accepted")}
	if v,_:=m.Get("server","theme.color");v!="green"{t.Fatal("invalid import changed active snapshot")}
	if _,err=LoadProject(entry,nil,ProjectOptions{Root:root,Profile:"missing"});err==nil{t.Fatal("unknown profile accepted")}
}

func TestProjectRejectsUnsafeAndAmbiguousImports(t *testing.T){
	root:=t.TempDir()
	entry:=filepath.Join(root,"server.scl")
	part:=filepath.Join(root,"other.scl")
	write:=func(path,s string){t.Helper();if err:=os.WriteFile(path,[]byte(s),0600);err!=nil{t.Fatal(err)}}
	for _,tc:=range []struct{entry,part string}{
		{"imports = [\"../outside.scl\"]\n",""},
		{"imports = [\"/tmp/outside.scl\"]\n",""},
		{"imports = [\"other.scl\", \"other.scl\"]\n","x = 1\n"},
		{"imports = [\"other.scl\"]\n","imports = [\"server.scl\"]\n"},
		{"imports = [\"other.scl\"]\nx = 1\n","x = 1\n"},
		{"imports = [\"missing.scl\"]\n",""},
		{"imports = [\"other.scl\"]\n","x = unknown\n"},
	}{
		write(entry,tc.entry);write(part,tc.part)
		if _,err:=LoadProject(entry,nil,ProjectOptions{Root:root});err==nil{t.Fatalf("accepted %s / %s",tc.entry,tc.part)}
	}
	write(entry,"imports = [\"link.scl\"]\n")
	if err:=os.Symlink(part,filepath.Join(root,"link.scl"));err==nil{
		if _,err=LoadProject(entry,nil,ProjectOptions{Root:root});err==nil{t.Fatal("accepted symlink")}
	}
	if _,err:=Load(entry,nil);err==nil{t.Fatal("plain Load silently ignored imports")}
}
