package scl

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExtendedSchema(t *testing.T) {
	for _, tc:=range []struct{kind Kind; good,bad string}{
		{IP,"\"::1\"","\"invalid\""},
		{CIDR,"\"192.0.2.0/24\"","\"192.0.2.0/99\""},
		{Port,"2222","65536"},
		{Hostname,"\"example.com\"","\"-bad.example\""},
		{URL,"\"https://example.com\"","\"file:///tmp/a\""},
		{Email,"\"user@example.com\"","\"not-an-email\""},
		{Color,"\"#ABCDEF\"","\"#XYZXYZ\""},
		{Percentage,"42.5","101"},
		{Bytes,"4096","-1"},
	}{
		s:=Schema{"value":{Kind:tc.kind}}
		if _,err:=Parse([]byte("value = "+tc.good+"\n"),"test.conf",s);err!=nil{t.Errorf("%s rejected: %v",tc.kind,err)}
		if _,err:=Parse([]byte("value = "+tc.bad+"\n"),"test.conf",s);err==nil{t.Errorf("%s accepted invalid value",tc.kind)}
	}
	s:=Schema{"colors":{Kind:Array,Element:Color,MinLength:1,MaxLength:2},"name":{Kind:String,Pattern:"^[a-z]+$",MaxLength:8}}
	if _,err:=Parse([]byte("colors = [\"#ABCDEF\"]\nname = \"demo\"\n"),"test.conf",s);err!=nil{t.Fatal(err)}
	for _,src:=range []string{"colors = [1]\n","colors = []\n","name = \"BAD\"\n","name = \"longerthan8\"\n"}{
		if _,err:=Parse([]byte(src),"test.conf",s);err==nil{t.Fatalf("accepted invalid constraint: %s",src)}
	}
	if _,err:=Parse(nil,"test.conf",Schema{"x":{Kind:"unknown"}});err==nil{t.Fatal("invalid optional schema accepted")}
}

func TestChangeReportAndNotifications(t *testing.T) {
	p:=filepath.Join(t.TempDir(),"test.scl")
	write:=func(s string){t.Helper();if err:=os.WriteFile(p,[]byte(s),0600);err!=nil{t.Fatal(err)}}
	write("theme {\n color = \"old\"\n}\n")
	m,err:=OpenManager([]FileSpec{{Name:"theme",Path:p}})
	if err!=nil{t.Fatal(err)}
	ch,cancel:=m.Subscribe()
	defer cancel()
	write("theme {\n color = \"new\"\n fps = 30\n}\n")
	check,err:=m.CheckReload()
	if err!=nil||len(check.Changes)!=2{t.Fatalf("check: %+v %v",check,err)}
	select {case <-ch:t.Fatal("preflight emitted");default:}
	r,err:=m.Reload()
	if err!=nil||len(r.Changes)!=2||!reflect.DeepEqual(r.Changes[0].Path,[]string{"theme","color"})||r.Changes[1].Kind!="added"{t.Fatalf("reload: %+v %v",r,err)}
	select {case g:=<-ch:if g!=2{t.Fatal(g)};default:t.Fatal("notification missing")}
	if _,err=m.Reload();err!=nil{t.Fatal(err)}
	select{case <-ch:t.Fatal("no-op emitted");default:}
	write("invalid = unknown\n")
	if _,err=m.Reload();err==nil{t.Fatal("invalid accepted")}
	select{case <-ch:t.Fatal("failure emitted");default:}
	cancel()
	if _,ok:=<-ch;ok{t.Fatal("subscription not closed")}
}
