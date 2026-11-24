package scl

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Revision identifies a source snapshot by its SHA-256 content digest.
// Created is the backup file modification time, not an audit timestamp.
type Revision struct {
	ID string
	Created time.Time
	Size int64
}

const MaxHistoryEntries = 4096

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validRevision(id string) bool {
	if len(id)!=64 || id!=strings.ToLower(id){return false}
	_,err:=hex.DecodeString(id)
	return err==nil
}

// historyPath scopes backups to the absolute document path. The caller must
// supply a trusted directory outside public or runtime configuration trees.
func (d *Document) historyPath(root string, create bool)(string,error){
	if d.path==""||root==""{return "",fmt.Errorf("document path and history directory are required")}
	root,err:=filepath.Abs(root)
	if err!=nil{return "",err}
	dir:=filepath.Join(root,digest([]byte(d.path)))
	for _,p:=range []string{root,dir}{
		if create {
			if err=os.MkdirAll(p,0700);err!=nil{return "",err}
		}
		info,err:=os.Lstat(p)
		if err!=nil{return "",err}
		if !info.IsDir()||info.Mode()&os.ModeSymlink!=0{return "",fmt.Errorf("history directory must not be a symlink")}
		// Windows ACLs cannot be inspected using Unix permission bits.
		if filepath.Separator!='\\' && info.Mode().Perm()&0077!=0{return "",fmt.Errorf("history directory must be private (0700)")}
	}
	return dir,nil
}

func readRevision(dir,id string)([]byte,error){
	if !validRevision(id){return nil,fmt.Errorf("invalid revision identifier")}
	path:=filepath.Join(dir,id+".scl")
	info,err:=os.Lstat(path)
	if err!=nil{return nil,err}
	if filepath.Separator!='\\' && info.Mode().Perm()&0077!=0{return nil,fmt.Errorf("revision must be private")}
	raw,err:=readRegular(path)
	if err!=nil{return nil,err}
	if digest(raw)!=id{return nil,fmt.Errorf("revision integrity check failed")}
	return raw,nil
}

func (d *Document) backupLocked(root string)(Revision,error){
	if d.path==""{return Revision{},fmt.Errorf("document has no file")}
	raw,err:=readRegular(d.path)
	if err!=nil{return Revision{},err}
	if !bytes.Equal(raw,d.original){return Revision{},fmt.Errorf("configuration changed on disk; reload first")}
	dir,err:=d.historyPath(root,true)
	if err!=nil{return Revision{},err}
	id:=digest(raw)
	path:=filepath.Join(dir,id+".scl")
	if _,err=os.Lstat(path);err==nil{
		if _,err=readRevision(dir,id);err!=nil{return Revision{},err}
	}else if !os.IsNotExist(err){return Revision{},err
	}else{
		// Exclusive creation never overwrites an existing revision. A crash
		// can leave a partial file, which is rejected by the hash check.
		f,err:=os.OpenFile(path,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600)
		if err!=nil{return Revision{},err}
		_,writeErr:=f.Write(raw)
		if writeErr==nil{writeErr=f.Sync()}
		closeErr:=f.Close()
		if writeErr==nil{writeErr=closeErr}
		if writeErr!=nil{_ = os.Remove(path);return Revision{},writeErr}
	}
	info,err:=os.Lstat(path)
	if err!=nil{return Revision{},err}
	return Revision{ID:id,Created:info.ModTime().UTC(),Size:info.Size()},nil
}

// Backup archives the last loaded on-disk source, not unsaved edits.
func (d *Document) Backup(root string)(Revision,error){
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.backupLocked(root)
}

// SaveWithHistory archives the previous source before saving the candidate.
// If backup fails, the current file is not overwritten.
func (d *Document) SaveWithHistory(root string)(Revision,error){
	d.mu.Lock()
	defer d.mu.Unlock()
	rev,err:=d.backupLocked(root)
	if err!=nil{return Revision{},err}
	return rev,d.saveLocked()
}

// History lists metadata, newest first. Revision contents are not returned.
func (d *Document) History(root string)([]Revision,error){
	d.mu.RLock()
	defer d.mu.RUnlock()
	dir,err:=d.historyPath(root,false)
	if os.IsNotExist(err){return nil,nil}
	if err!=nil{return nil,err}
	f,err:=os.Open(dir)
	if err!=nil{return nil,err}
	defer f.Close()
	var out []Revision
	total:=0
	for{
		entries,e:=f.ReadDir(128)
		for _,entry:=range entries{
			total++
			if total>MaxHistoryEntries{return nil,fmt.Errorf("history entry limit exceeded")}
			id:=strings.TrimSuffix(entry.Name(),".scl")
			if !strings.HasSuffix(entry.Name(),".scl")||!validRevision(id){continue}
			info,err:=entry.Info()
			if err!=nil{return nil,err}
			if !info.Mode().IsRegular()||info.Size()>MaxBytes{return nil,fmt.Errorf("invalid history entry %s",id)}
			out=append(out,Revision{ID:id,Created:info.ModTime().UTC(),Size:info.Size()})
		}
		if e==io.EOF{break}
		if e!=nil{return nil,e}
	}
	sort.Slice(out,func(i,j int)bool{
		if out[i].Created.Equal(out[j].Created){return out[i].ID<out[j].ID}
		return out[i].Created.After(out[j].Created)
	})
	return out,nil
}

// Restore checks integrity and validates against the current schema before
// archiving the existing source and saving the revision. Unsaved changes are
// rejected. In-memory changes are rolled back if saving fails.
func (d *Document) Restore(root,id string)(Revision,error){
	d.mu.Lock()
	defer d.mu.Unlock()
	if !bytes.Equal(d.source,d.original){return Revision{},fmt.Errorf("unsaved changes")}
	dir,err:=d.historyPath(root,false)
	if err!=nil{return Revision{},err}
	raw,err:=readRevision(dir,id)
	if err!=nil{return Revision{},err}
	candidate,err:=Parse(raw,d.path,d.schema)
	if err!=nil{return Revision{},err}
	rev,err:=d.backupLocked(root)
	if err!=nil{return Revision{},err}
	oldSource,oldValues:=d.source,d.values
	d.source,d.values=candidate.source,candidate.values
	if err=d.saveLocked();err!=nil{
		d.source,d.values=oldSource,oldValues
		return rev,err
	}
	return rev,nil
}

// Diff compares effective settings without exposing values. Formatting and
// comment-only changes do not appear in the result.
func Diff(before,after *Document)[]Change{
	return diffObjects("",nil,before.Snapshot(),after.Snapshot())
}

// DiffRevision compares a verified archived revision with current effective
// settings using the current schema. It does not modify either source.
func (d *Document) DiffRevision(root,id string)([]Change,error){
	d.mu.RLock()
	defer d.mu.RUnlock()
	dir,err:=d.historyPath(root,false)
	if err!=nil{return nil,err}
	raw,err:=readRevision(dir,id)
	if err!=nil{return nil,err}
	old,err:=Parse(raw,d.path,d.schema)
	if err!=nil{return nil,err}
	return diffObjects("",nil,old.values,d.values),nil
}
