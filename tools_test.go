package scl

import (
	"strings"
	"testing"
)

func TestExplicitSecretReferences(t *testing.T){
	t.Setenv("SCL_TEST_TOKEN","secret-value")
	t.Setenv("SCL_OTHER_TOKEN","not-allowed")
	s:=Schema{"token":{Kind:Secret,Required:true}}
	d,err:=Parse([]byte("token = \"env:SCL_TEST_TOKEN\"\n"),"test.scl",s)
	if err!=nil{t.Fatal(err)}
	if d.String("token")!="env:SCL_TEST_TOKEN"{t.Fatal("secret resolved during parsing")}
	r,err:=NewEnvResolver("SCL_TEST_TOKEN")
	if err!=nil{t.Fatal(err)}
	if v,e:=r.Resolve(d.String("token"));e!=nil||v!="secret-value"{t.Fatal("explicit resolution failed")}
	if _,err=r.Resolve("env:SCL_OTHER_TOKEN");err==nil{t.Fatal("unapproved secret exposed")}
	if _,err=Parse([]byte("token = \"plaintext\"\n"),"test.scl",s);err==nil{t.Fatal("literal secret accepted")}
}

func TestGenerateAndDescribe(t *testing.T){
	s:=Schema{
		"server.name":{Kind:String,Default:"demo"},
		"server.fps":{Kind:Integer,Default:30},
		"server.secret":{Kind:String,Sensitive:true,Default:"private-value"},
		"server.token":{Kind:Secret,Required:true},
	}
	out,err:=s.Generate()
	if err!=nil{t.Fatal(err)}
	if strings.Contains(string(out),"private-value"){t.Fatal("sensitive default exported")}
	d,err:=Parse(out,"generated.scl",nil)
	if err!=nil||d.String("server.name")!="demo"{t.Fatalf("invalid starter: %v",err)}
	docs,err:=s.Describe()
	if err!=nil||strings.Contains(string(docs),"private-value"){t.Fatal("documentation leaks values")}
	var dest struct{Server struct{Name string;FPS int}}
	if err=d.Decode(&dest);err!=nil||dest.Server.FPS!=30{t.Fatalf("decode: %v",err)}
}

func FuzzParse(f *testing.F){
	f.Add([]byte("name = \"demo\"\n"))
	f.Add([]byte("preset base { fps = 30 }\nui { use = \"base\" }\n"))
	f.Fuzz(func(t *testing.T,src []byte){
		if len(src)>MaxBytes{t.Skip()}
		d,err:=Parse(src,"fuzz.scl",nil)
		if err!=nil{return}
		_ = d.Snapshot()
		if err=d.Format();err!=nil{t.Fatalf("valid source failed to format: %v",err)}
	})
}

func BenchmarkParse(b *testing.B){
	src:=[]byte("theme {\n name = \"demo\"\n colors = [\"#FFFFFF\", \"#000000\"]\n fps = 30\n}\n")
	b.ReportAllocs()
	for i:=0;i<b.N;i++{
		if _,err:=Parse(src,"bench.scl",nil);err!=nil{b.Fatal(err)}
	}
}
