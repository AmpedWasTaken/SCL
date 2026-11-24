package scl

import (
	"encoding/json"
	"fmt"
	"net"
	"net/mail"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

var hostnameLabel = regexp.MustCompile("^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$")
var rgbColor = regexp.MustCompile("^#[0-9A-Fa-f]{6}$")

func validHostname(s string) bool {
	s = strings.TrimSuffix(s, ".")
	if len(s) == 0 || len(s) > 253 { return false }
	for _, label := range strings.Split(s, ".") {
		if !hostnameLabel.MatchString(label) { return false }
	}
	return true
}

func checkExtendedKind(kind Kind, v any) bool {
	switch kind {
	case Port, Bytes, Percentage:
		n, ok := v.(json.Number)
		if !ok { return false }
		if kind == Percentage {
			f, err := n.Float64()
			return err == nil && f >= 0 && f <= 100
		}
		i, err := n.Int64()
		return err == nil && i >= 0 && (kind == Bytes || (i >= 1 && i <= 65535))
	}
	s, ok := v.(string)
	if !ok { return false }
	switch kind {
	case IP:
		ip, err := netip.ParseAddr(s)
		return err == nil && ip.Zone() == ""
	case CIDR:
		_, err := netip.ParsePrefix(s)
		return err == nil
	case Hostname:
		return validHostname(s)
	case URL:
		u, err := url.Parse(s)
		return err == nil && (u.Scheme == "https" || u.Scheme == "http") &&
			u.User == nil && u.Hostname() != "" &&
			(validHostname(u.Hostname()) || net.ParseIP(u.Hostname()) != nil)
	case Email:
		a, err := mail.ParseAddress(s)
		return err == nil && a.Name == "" && a.Address == s
	case Color:
		return rgbColor.MatchString(s)
	case Secret:
		_,err:=parseSecretRef(s)
		return err==nil
	default:
		return false
	}
}

func knownKind(k Kind) bool {
	switch k {
	case String,Integer,Number,Boolean,Array,Object,Duration,IP,CIDR,Port,Hostname,URL,Email,Color,Percentage,Bytes,Secret:
		return true
	}
	return false
}

func validateFieldDefinition(path string, f Field) error {
	for _, part := range strings.Split(path, ".") {
		if !validSegment(part) { return fmt.Errorf("invalid schema path %q",path) }
	}
	if !knownKind(f.Kind) { return fmt.Errorf("%s: unknown schema kind",path) }
	if f.Element != "" && (f.Kind != Array || !knownKind(f.Element)) { return fmt.Errorf("%s: invalid array element type",path) }
	if f.MinLength < 0 || f.MaxLength < 0 || (f.MaxLength > 0 && f.MinLength > f.MaxLength) { return fmt.Errorf("%s: invalid length constraints",path) }
	if (f.Min != nil || f.Max != nil) && f.Kind != Integer { return fmt.Errorf("%s: integer bounds require integer kind",path) }
	if f.Min != nil && f.Max != nil && *f.Min > *f.Max { return fmt.Errorf("%s: invalid integer bounds",path) }
	if f.Kind==Secret && (f.Default!=nil || len(f.Enum)>0) { return fmt.Errorf("%s: secrets cannot have defaults or enum constraints",path) }
	if f.Pattern != "" {
		if f.Kind != String { return fmt.Errorf("%s: pattern requires string kind",path) }
		if _,err:=regexp.Compile(f.Pattern);err!=nil { return fmt.Errorf("%s: invalid schema pattern",path) }
	}
	return nil
}

func checkConstraints(path string, f Field, v any) error {
	if f.Pattern != "" {
		s, ok := v.(string)
		p, err := regexp.Compile(f.Pattern)
		if !ok || err != nil || !p.MatchString(s) { return fmt.Errorf("%s: value does not match schema pattern",path) }
	}
	if f.MinLength > 0 || f.MaxLength > 0 {
		n := -1
		switch x:=v.(type) {
		case string: n=utf8.RuneCountInString(x)
		case []any: n=len(x)
		case object: n=len(x)
		}
		if n < f.MinLength || (f.MaxLength > 0 && n > f.MaxLength) { return fmt.Errorf("%s: length outside permitted bounds",path) }
	}
	if f.Element != "" {
		items, ok:=v.([]any)
		if !ok { return fmt.Errorf("%s: expected array",path) }
		for i,item:=range items {
			if err:=checkField(fmt.Sprintf("%s[%d]",path,i),Field{Kind:f.Element},item);err!=nil { return err }
		}
	}
	return nil
}
