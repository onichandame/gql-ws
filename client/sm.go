package gqlwsclient

import (
	"errors"
	"sync"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/gqlerrors"
)

type subMan struct {
	subs map[string]*Handlers
	lock sync.RWMutex
}

func newSubMan() *subMan {
	var sm subMan
	sm.subs = make(map[string]*Handlers)
	return &sm
}

func (sm *subMan) set(id string, hdl *Handlers) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	if _, ok := sm.subs[id]; ok {
		panic(errors.New(`subscription already present`))
	}
	sm.subs[id] = hdl
}

func (sm *subMan) get(id string) *Handlers {
	sm.lock.RLock()
	defer sm.lock.RUnlock()
	return sm.subs[id]
}

func (sm *subMan) del(id string) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	delete(sm.subs, id)
}

type Handlers struct {
	OnError    func(gqlerrors.FormattedErrors)
	OnComplete func()
	OnNext     func(*graphql.Result)
}
