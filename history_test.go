package scl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHistorySaveRestoreAndDiff(t *testing.T){
	root:=t.TempDir()
	path:=filepath.Join(root,"server.scl")
	history:=filepath.Join(root,"history")
	if err:=os.WriteFile(path,[]byte("# initial\nname = \"old\"\n"),0600);err!=nil{t.Fatal(err)}
	d,err:=Load(path,Schema{"name":{Kind:String,Required:true}})
	if err!=nil{t.Fatal(err)}
	if err=d.Set("name","new");err!=nil{t.Fatal(err)}
	old,err:=d.SaveWithHistory(history)
	if err!=nil{t.Fatal(err)}
	if !validRevision(old.ID){t.Fatal("invalid revision ID")}
	diffs,err:=d.DiffRevision(history,old.ID)
	if err!=nil||len(diffs)!=1||diffs[0].Kind!="modified"||diffs[0].Path[0]!="name"{t.Fatalf("diff: %+v %v",diffs,err)}
	newRev,err:=d.Restore(history,old.ID)
	if err!=nil{t.Fatal(err)}
	if d.String("name")!="old"{t.Fatal("restore not activated")}
	data,err:=os.ReadFile(path)
	if err!=nil||string(data)!="# initial\nname = \"old\"\n"{t.Fatal("source not restored")}
	revisions,err:=d.History(history)
	if err!=nil||len(revisions)!=2{t.Fatalf("history: %+v %v",revisions,err)}
	if _,err=d.Restore(history,newRev.ID);err!=nil||d.String("name")!="new"{t.Fatal("restore undo failed")}
	if _,err=d.Backup(history);err!=nil{t.Fatal(err)}
	revisions,err=d.History(history)
	if err!=nil||len(revisions)!=2{t.Fatal("identical backups not deduplicated")}
}

func TestHistoryRejectsTamperingAndExternalEdits(t *testing.T){
	root:=t.TempDir()
	p:=filepath.Join(root,"file.scl")
	h:=filepath.Join(root,"history")
	if err:=os.WriteFile(p,[]byte("x = 1\n"),0600);err!=nil{t.Fatal(err)}
	d,err:=Load(p,nil)
	if err!=nil{t.Fatal(err)}
	rev,err:=d.Backup(h)
	if err!=nil{t.Fatal(err)}
	dir,err:=d.historyPath(h,false)
	if err!=nil{t.Fatal(err)}
	if err=os.WriteFile(filepath.Join(dir,rev.ID+".scl"),[]byte("x = 2\n"),0600);err!=nil{t.Fatal(err)}
	if _,err=d.Restore(h,rev.ID);err==nil{t.Fatal("corrupt revision accepted")}
	if _,err=d.Restore(h,"../outside");err==nil{t.Fatal("invalid ID accepted")}
	if err=d.Set("x",3);err!=nil{t.Fatal(err)}
	if _,err=d.SaveWithHistory(h);err==nil{t.Fatal("failed backup did not block save")}
	b,err:=os.ReadFile(p)
	if err!=nil||string(b)!="x = 1\n"{t.Fatal("failed backup overwrote file")}
	if err=os.WriteFile(p,[]byte("x = 4\n"),0600);err!=nil{t.Fatal(err)}
	if _,err=d.SaveWithHistory(h);err==nil{t.Fatal("external edit overwritten")}
}

func TestRestoreValidatesCurrentSchema(t *testing.T){
	root:=t.TempDir()
	p:=filepath.Join(root,"file.scl")
	h:=filepath.Join(root,"history")
	if err:=os.WriteFile(p,[]byte("x = \"old\"\n"),0600);err!=nil{t.Fatal(err)}
	d,err:=Load(p,nil)
	if err!=nil{t.Fatal(err)}
	old,err:=d.Backup(h)
	if err!=nil{t.Fatal(err)}
	if err=os.WriteFile(p,[]byte("x = 42\n"),0600);err!=nil{t.Fatal(err)}
	strict,err:=Load(p,Schema{"x":{Kind:Integer}})
	if err!=nil{t.Fatal(err)}
	if _,err=strict.Restore(h,old.ID);err==nil||strict.Int("x")!=42{t.Fatal("invalid revision applied")}
}
