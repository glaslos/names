package main

import (
	"time"

	"github.com/glaslos/names"
	"github.com/glaslos/names/cache"
)

func main() {
	config := names.Config{
		ListenerAddress: "127.0.0.1:53",
		CacheConfig: &cache.Config{
			ExpirationTime:  (100 * time.Millisecond).Nanoseconds(),
			RefreshInterval: 10 * time.Second,
			Persist:         false,
		},
		LoggerConfig: &names.LoggerConfig{
			Filename:   "./names.log",
			MaxSize:    500, // megabytes
			MaxBackups: 3,
			MaxAge:     28,   //days
			Compress:   true, // disabled by default
		},
		DNSClientNet:     "tcp-tls",
		DNSClientTimeout: time.Duration(10) * time.Second,
	}
	n, err := names.New(&config)
	if err != nil {
		panic(err)
	}
	n.Run()
	n.Log.Printf("exiting.\n")
}
