package cache

import (
	"encoding/gob"
	"os"
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
	Persist         bool
	DumpInterval    time.Duration
}

// Cache struct
type Cache struct {
	Elements map[string]Element
	mutex    sync.RWMutex
	config   *Config
}

// Save the cache to a file
func (cache *Cache) Save(path string) error {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	gob.Register(dns.A{})
	gob.Register(dns.CNAME{})
	fh, err := os.Create(path)
	defer fh.Close()
	if err != nil {
		return err
	}
	return gob.NewEncoder(fh).Encode(cache.Elements)
}

// Load the cache from a cache
func (cache *Cache) Load(path string) error {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	fh, err := os.Open(path)
	defer fh.Close()
	if err != nil {
		return err
	}
	gob.Register(dns.A{})
	gob.Register(dns.CNAME{})
	return gob.NewDecoder(fh).Decode(&cache.Elements)
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

func (cache *Cache) dump() {
	tick := time.Tick(cache.config.DumpInterval)
	for {
		select {
		case <-tick:
			if err := cache.Save("cache.dump"); err != nil {
				println(err.Error())
			}
		}
	}
}

// New initializes the cache
func New(config Config) (*Cache, error) {
	cache := &Cache{
		Elements: make(map[string]Element),
		config:   &config,
	}
	if config.Persist {
		if err := cache.Load("cache.dump"); err != nil {
			if !os.IsNotExist(err) {
				return cache, err
			}
			// Ignoring error of missing cache dump
		}
		if config.DumpInterval == 0 {
			config.DumpInterval = 60 * time.Second
		}
		go cache.dump()
	}
	if config.RefreshFunc != nil && config.RefreshInterval > 0 {
		go cache.refresh()
	}
	return cache, nil
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
