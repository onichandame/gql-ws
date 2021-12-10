package gqlws

import "sync"

type subscriptionManager struct {
	subscriptions map[string]chan interface{}
	lock          sync.RWMutex
}

func newSubscriptionManager() *subscriptionManager {
	var sm subscriptionManager
	sm.subscriptions = make(map[string]chan interface{})
	return &sm
}

func (sm *subscriptionManager) add(id string) chan interface{} {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	sm.subscriptions[id] = make(chan interface{})
	return sm.subscriptions[id]
}

func (sm *subscriptionManager) del(id string) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	delete(sm.subscriptions, id)
}

func (sm *subscriptionManager) has(id string) bool {
	sm.lock.RLock()
	defer sm.lock.RUnlock()
	_, ok := sm.subscriptions[id]
	return ok
}

func (sm *subscriptionManager) get(id string) chan interface{} {
	sm.lock.RLock()
	defer sm.lock.RUnlock()
	return sm.subscriptions[id]
}
