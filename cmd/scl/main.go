package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AmpedWasTaken/SCL"
)

func run(args []string)error{
	if len(args)<2{return fmt.Errorf("usage: scl validate FILE | fmt FILE [--write] | set FILE PATH JSON | migrate-json FILE")}
	command,path:=args[0],args[1]
	if command=="migrate-json"{
		if len(args)!=2{return fmt.Errorf("usage: scl migrate-json FILE")}
		f,err:=os.Open(path);if err!=nil{return err};defer f.Close()
		info,err:=f.Stat();if err!=nil{return err}
		if !info.Mode().IsRegular()||info.Size()>scl.MaxBytes{return fmt.Errorf("invalid input file")}
		raw,err:=io.ReadAll(io.LimitReader(f,scl.MaxBytes+1));if err!=nil{return err}
		out,err:=scl.FromJSON(raw);if err!=nil{return err}
		_,err=os.Stdout.Write(out);return err
	}
	d,err:=scl.Load(path,nil);if err!=nil{return err}
	switch command{
	case "validate":
		if len(args)!=2{return fmt.Errorf("usage: scl validate FILE")}
		fmt.Println("Valid SCL (syntax and preset resolution)")
	case "fmt":
		if len(args)>3||(len(args)==3&&args[2]!="--write"){return fmt.Errorf("usage: scl fmt FILE [--write]")}
		if err=d.Format();err!=nil{return err}
		if len(args)==3{return d.Save()}
		_,err=os.Stdout.Write(d.Bytes());return err
	case "set":
		if len(args)!=4{return fmt.Errorf("usage: scl set FILE PATH JSON")}
		dec:=json.NewDecoder(strings.NewReader(args[3]));dec.UseNumber()
		var v any
		if !json.Valid([]byte(args[3])){return fmt.Errorf("expected one JSON value")}
		if err=dec.Decode(&v);err!=nil{return err}
		if err=d.Set(args[2],v);err!=nil{return err};return d.Save()
	default:return fmt.Errorf("unknown command %q",command)
	}
	return nil
}
func main(){if err:=run(os.Args[1:]);err!=nil{fmt.Fprintln(os.Stderr,"scl:",err);os.Exit(1)}}
