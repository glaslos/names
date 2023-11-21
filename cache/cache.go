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
	Value     string
	Refresh   bool
	TimeAdded time.Time
	Resolver  string
	Request   []byte
}

// Config for the cache
type Config struct {
	ExpirationTime  time.Duration
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
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	gob.Register(dns.A{})
	gob.Register(dns.CNAME{})
	return gob.NewEncoder(fh).Encode(cache.Elements)
}

// Load the cache from a cache
func (cache *Cache) Load(path string) error {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	fh, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	gob.Register(dns.A{})
	gob.Register(dns.CNAME{})
	return gob.NewDecoder(fh).Decode(&cache.Elements)
}

func (cache *Cache) refresh() {
	ticker := time.NewTicker(cache.config.RefreshInterval)
	for range ticker.C {
		cache.config.RefreshFunc(cache)
	}
}

func (cache *Cache) dump() {
	ticker := time.NewTicker(cache.config.DumpInterval)
	for range ticker.C {
		if err := cache.Save("cache.dump"); err != nil {
			println(err.Error())
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

// Get an element from the cache
func (cache *Cache) Get(k string) (*Element, bool) {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	element, found := cache.Elements[k]
	if !found {
		return nil, false
	}
	if cache.config.ExpirationTime > 0 {
		// adding the negative expiration time
		if time.Now().Add(time.Duration(-cache.config.ExpirationTime)).After(element.TimeAdded) {
			return nil, false
		}
	}
	return &element, true
}

// Set an element in the cache
func (cache *Cache) Set(k string, v Element) {
	cache.mutex.Lock()

	v.TimeAdded = time.Now()
	cache.Elements[k] = v

	cache.mutex.Unlock()
}
