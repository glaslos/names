package cache

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Element in the cache
type Element struct {
	Value     []dns.RR
	Refresh   bool
	TimeAdded int64
	Resolver  string
	Request   *dns.Msg
}

// Config for the cache
type Config struct {
	ExpirationTime  int64
	RefreshInterval time.Duration
	RefreshFunc     func(cache *Cache)
}

// Cache struct
type Cache struct {
	Elements map[string]Element
	mutex    sync.RWMutex
	config   Config
}

func (cache *Cache) refresh() {
	tick := time.Tick(cache.config.RefreshInterval)
	for {
		select {
		case <-tick:
			cache.config.RefreshFunc(cache)
		}
	}
}

// New initializes the cache
func New(config Config) *Cache {
	cache := &Cache{
		Elements: make(map[string]Element),
		config:   config,
	}
	if config.RefreshFunc != nil && config.RefreshInterval > 0 {
		go cache.refresh()
	}
	return cache
}

func (cache *Cache) evict() {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	for k, v := range cache.Elements {
		if time.Now().UnixNano()-cache.config.ExpirationTime > v.TimeAdded {
			// deleting the expired element
			delete(cache.Elements, k)
		}
	}
}

// Get an element from the cache
func (cache *Cache) Get(k string) (*Element, bool) {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	element, found := cache.Elements[k]
	if !found {
		return nil, false
	}
	if cache.config.ExpirationTime > 0 {
		if time.Now().UnixNano()-cache.config.ExpirationTime > element.TimeAdded {
			return nil, false
		}
	}
	return &element, true
}

// Set an element in the cache
func (cache *Cache) Set(k string, v Element) {
	cache.mutex.Lock()

	v.TimeAdded = time.Now().UnixNano()
	cache.Elements[k] = v

	cache.mutex.Unlock()
}
