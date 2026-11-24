package scl

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// A secret field stores an opaque reference, never the resolved value.
// Environment reads happen only through this explicit host-controlled API.
var secretName = regexp.MustCompile("^[A-Z_][A-Z0-9_]{0,127}$")

func parseSecretRef(ref string)(string,error){
	name,ok:=strings.CutPrefix(ref,"env:")
	if !ok||!secretName.MatchString(name){return "",fmt.Errorf("invalid secret reference (expected env:NAME)")}
	return name,nil
}

// EnvResolver resolves only names explicitly allowed by the host application.
// Do not derive the allowlist from a configuration file. Parsing, formatting,
// documentation and reload never invoke this resolver.
type EnvResolver struct {
	allowed map[string]bool
}

func NewEnvResolver(allowed ...string)(*EnvResolver,error){
	r:=&EnvResolver{allowed:map[string]bool{}}
	for _,name:=range allowed{
		if !secretName.MatchString(name){return nil,fmt.Errorf("invalid allowlisted environment name")}
		r.allowed[name]=true
	}
	return r,nil
}

func (r *EnvResolver) Resolve(ref string)(string,error){
	name,err:=parseSecretRef(ref)
	if err!=nil{return "",err}
	if r==nil||!r.allowed[name]{return "",fmt.Errorf("secret reference is not allowlisted")}
	value,ok:=os.LookupEnv(name)
	if !ok||value==""{return "",fmt.Errorf("referenced secret is unavailable")}
	return value,nil
}
