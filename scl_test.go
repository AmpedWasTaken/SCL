package scl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const example = "# Keep this comment\npreset animated {\n  fps = 30\n}\ntheme midnight {\n  animation {\n    use = \"animated\"\n  }\n  colors = [\"#aaa\", \"#bbb\"]\n}\n"

func TestPresetsAndEdits(t *testing.T){
	s:=Schema{"theme.midnight.animation.fps":{Kind:Integer,Required:true},"theme.midnight.colors":{Kind:Array}}
	d,err:=Parse([]byte(example),"test.conf",s);if err!=nil{t.Fatal(err)}
	if d.Int("theme.midnight.animation.fps")!=30{t.Fatal("preset not applied")}
	if err=d.Set("theme.midnight.animation.fps",60);err!=nil{t.Fatal(err)}
	if d.Int("theme.midnight.animation.fps")!=60{t.Fatal("override not applied")}
	if !strings.Contains(string(d.Bytes()),"# Keep this comment"){t.Fatal("comment lost")}
	before:=string(d.Bytes())
	if err=d.Set("theme.midnight.animation.fps","bad");err==nil||string(d.Bytes())!=before{t.Fatal("invalid edit accepted")}
	v:=d.Snapshot()
	delete(v,"theme")
	if d.Int("theme.midnight.animation.fps")!=60{t.Fatal("snapshot not detached")}
}

func TestRejectInvalid(t *testing.T){
	for _,src:=range []string{
		"x = file(\"secret\")\n","x = var.name\n","x = 1+2\n","x = \"${1}\"\n",
		"x=1\nx=2\n","a {}\na {}\n","preset a { use = \"b\" }\npreset b { use = \"a\" }\n",
		"theme a { use = \"missing\" }\n",
	}{
		if _,err:=Parse([]byte(src),"test.conf",nil);err==nil{t.Errorf("accepted %q",src)}
	}
}

func TestSchemaDefaultsAndUnknown(t *testing.T){
	s:=Schema{"server.name":{Kind:String,Default:"default"},"server.enabled":{Kind:Boolean,Required:true}}
	d,err:=Parse([]byte("server { enabled = true }\n"),"test.conf",s)
	if err!=nil||d.String("server.name")!="default"{t.Fatalf("defaults: %v",err)}
	if _,err=Parse([]byte("server { extra = true }\n"),"test.conf",s);err==nil{t.Fatal("invalid schema accepted")}
}

func TestSaveAndReload(t *testing.T){
	p:=filepath.Join(t.TempDir(),"example.conf")
	if err:=os.WriteFile(p,[]byte(example),0600);err!=nil{t.Fatal(err)}
	d,err:=Load(p,nil);if err!=nil{t.Fatal(err)}
	if err=d.Set("theme.midnight.animation.fps",60);err!=nil{t.Fatal(err)}
	if err=d.Save();err!=nil{t.Fatal(err)}
	if err=os.WriteFile(p,[]byte("bad = unknown\n"),0600);err!=nil{t.Fatal(err)}
	if err=d.Reload();err==nil||d.Int("theme.midnight.animation.fps")!=60{t.Fatal("invalid reload published")}
	if err=d.Set("theme.midnight.animation.fps",90);err!=nil{t.Fatal(err)}
	if err=d.Save();err==nil{t.Fatal("external modification overwritten")}
}

func TestJSONConversion(t *testing.T){
	out,err:=FromJSON([]byte("{\"server\":{\"name\":\"demo\",\"ports\":[80,443]}}"))
	if err!=nil{t.Fatal(err)}
	d,err:=Parse(out,"converted.conf",nil)
	if err!=nil||d.String("server.name")!="demo"{t.Fatalf("migration: %v",err)}
}
