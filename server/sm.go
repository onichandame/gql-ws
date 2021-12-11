package gqlwsserver

import "sync"

type subMan struct {
	subs map[string]chan interface{}
	lock sync.RWMutex
}

func newSubMan() *subMan {
	var sm subMan
	sm.subs = make(map[string]chan interface{})
	return &sm
}

func (sm *subMan) add(id string) chan interface{} {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	sm.subs[id] = make(chan interface{})
	return sm.subs[id]
}

func (sm *subMan) del(id string) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	sub := sm.subs[id]
	if sub != nil {
		close(sub)
		delete(sm.subs, id)
	}
}

func (sm *subMan) has(id string) bool {
	sm.lock.RLock()
	defer sm.lock.RUnlock()
	_, ok := sm.subs[id]
	return ok
}
