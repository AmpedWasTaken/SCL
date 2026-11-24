package scl

import (
	"reflect"
	"sort"
)

// Change describes an effective setting change. Values are deliberately not
// included. Path is segmented to avoid ambiguity with literal object keys.
// Arrays are treated as a single value; objects are compared recursively.
type Change struct {
	File string
	Path []string
	Kind string // added, removed, modified
}

func diffObjects(file string, prefix []string, old, next object) []Change {
	keys := map[string]bool{}
	for k := range old { keys[k] = true }
	for k := range next { keys[k] = true }
	ordered := make([]string,0,len(keys))
	for k := range keys { ordered = append(ordered,k) }
	sort.Strings(ordered)
	var changes []Change
	for _,k:=range ordered {
		a,aok:=old[k]; b,bok:=next[k]
		if aok==bok && reflect.DeepEqual(a,b) { continue }
		path:=append(append([]string(nil),prefix...),k)
		ao,am:=a.(object);bo,bm:=b.(object)
		if aok && bok && am && bm {
			changes=append(changes,diffObjects(file,path,ao,bo)...)
			continue
		}
		kind:="modified"
		if !aok { kind="added" } else if !bok { kind="removed" }
		changes=append(changes,Change{File:file,Path:path,Kind:kind})
	}
	return changes
}

// Subscribe returns a buffered stream of committed generation notifications
// and a cancellation function. Slow consumers receive the latest generation,
// not a guaranteed event history. Read Snapshot after notification.
// Failed checks and no-op reloads do not emit. Cancel closes the channel.
func (m *Manager) Subscribe() (<-chan uint64, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.subscribers==nil { m.subscribers=make(map[chan uint64]struct{}) }
	ch:=make(chan uint64,1)
	m.subscribers[ch]=struct{}{}
	return ch,func(){
		m.mu.Lock()
		defer m.mu.Unlock()
		if _,ok:=m.subscribers[ch];ok { delete(m.subscribers,ch);close(ch) }
	}
}

func (m *Manager) notifyLocked() {
	for ch:=range m.subscribers {
		select { case <-ch: default: }
		ch <- m.version
	}
}
