package gqlws

import "sync"

type subscriptionManager struct {
	subscriptions map[string]interface{}
	lock          sync.Mutex
}

func newSubscriptionManager() *subscriptionManager {
	var sm subscriptionManager
	sm.subscriptions = make(map[string]interface{})
	return &sm
}

func (sm *subscriptionManager) add(id string) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	sm.subscriptions[id] = nil
}

func (sm *subscriptionManager) del(id string) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	delete(sm.subscriptions, id)
}

func (sm *subscriptionManager) has(id string) bool {
	_, ok := sm.subscriptions[id]
	return ok
}
