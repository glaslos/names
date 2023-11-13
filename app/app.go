package main

import (
	"errors"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/glaslos/names"
	"github.com/glaslos/names/cache"
	"github.com/glaslos/names/lists"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func verifyAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if port == "" {
		return errors.New("port missing from address")
	}
	if _, err = strconv.ParseUint(port, 10, 16); err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip == nil {
		return errors.New("invalid IP address")
	}
	return nil
}

func main() {
	pflag.String("addr", "127.0.0.1:53", "Address the resolver listens on")
	pflag.String("dns-client-net", "tcp-tls", "Net to use for DNS requests")
	pflag.Duration("dns-client-timeout", 2*time.Second, "DNS client request timeout")
	pflag.Duration("cache-expiration", 10*time.Second, "Cache entry expiration in seconds")
	pflag.Duration("cache-dns-refresh", 60*time.Second, "Cache value refresh in seconds")
	pflag.Bool("cache-persist", false, "Set to persist cache to disk")
	pflag.String("log-file", "./names.log", "Path to log file")
	pflag.Int("log-max-size", 50, "Max log file size in MB")
	pflag.Int("log-file-retention", 3, "Number of log files to keep")
	pflag.Int("log-max-age", 28, "Max age of log files")
	pflag.Bool("log-compress", true, "Set to enable log file compression")
	pflag.StringSlice("fetch-lists", []string{"adguard"}, "Block lists to fetch")
	pflag.Bool("list-blocklists", false, "Set to list all block lists")
	pflag.StringSlice("upstreams", []string{"1.1.1.1:853", "9.9.9.9:853", "1.0.0.1:853", "8.8.4.4:853", "8.8.8.8:853"}, "Upstreams to resolve from")
	viper.BindPFlags(pflag.CommandLine)
	pflag.Parse()

	if viper.GetBool("list-blocklists") {
		listConfigs, err := lists.DecodeConfig()
		if err != nil {
			log.Fatal(err)
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Name", "Size", "Focus"})

		for name, config := range listConfigs {
			table.Append([]string{name, config.Size, config.Focus})
		}
		table.Render()
		os.Exit(0)
	}

	if err := verifyAddr(viper.GetString("addr")); err != nil {
		log.Fatal(err)
	}

	config := names.Config{
		ListenerAddress: viper.GetString("addr"),
		CacheConfig: &cache.Config{
			ExpirationTime:  viper.GetDuration("cache-expiration") * time.Second,
			RefreshInterval: viper.GetDuration("cache-dns-refresh") * time.Second,
			Persist:         viper.GetBool("cache-persist"),
		},
		LoggerConfig: &names.LoggerConfig{
			Filename:   viper.GetString("log-file"),
			MaxSize:    viper.GetInt("log-max-size"),
			MaxBackups: viper.GetInt("log-file-retention"),
			MaxAge:     viper.GetInt("log-max-age"),
			Compress:   viper.GetBool("log-compress"),
		},
		DNSClientNet:     viper.GetString("dns-client-net"),
		DNSClientTimeout: viper.GetDuration("dns-client-timeout") * time.Second,
	}
	n, err := names.New(&config)
	if err != nil {
		log.Fatal(err)
	}
	n.Run()
	n.Log.Printf("exiting.\n")
}
