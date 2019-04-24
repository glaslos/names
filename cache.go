package names

import (
	"github.com/miekg/dns"
	"sync"
	"time"
)

// Element in the cache
type Element struct {
	Value     []dns.RR
	Refresh   bool
	TimeAdded int64
	Resolver  string
}

// Cache struct
type Cache struct {
	elements       map[string]Element
	mutex          sync.RWMutex
	expirationTime int64
}

// InitCache initializes the cache
func InitCache(expirationTime int64) Cache {
	return Cache{
		elements:       make(map[string]Element),
		expirationTime: expirationTime,
	}
}

func (cache *Cache) evict() {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	for k, v := range cache.elements {
		if time.Now().UnixNano()-cache.expirationTime > v.TimeAdded {
			// deleting the expired element
			delete(cache.elements, k)
		}
	}
}

// Get an element from the cache
func (cache *Cache) Get(k string) (*Element, bool) {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	element, found := cache.elements[k]
	if !found {
		return nil, false
	}
	if cache.expirationTime > 0 {
		if time.Now().UnixNano()-cache.expirationTime > element.TimeAdded {
			return nil, false
		}
	}
	return &element, true
}

// Set an element in the cache
func (cache *Cache) Set(k string, v Element) {
	cache.mutex.Lock()

	v.TimeAdded = time.Now().UnixNano()
	cache.elements[k] = v

	cache.mutex.Unlock()
}
