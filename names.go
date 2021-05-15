package names

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/glaslos/names/cache"
	"github.com/spf13/viper"

	"github.com/glaslos/trie"
	"github.com/miekg/dns"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Names main struct
type Names struct {
	cache     *cache.Cache
	dnsClient dns.Client
	tree      *trie.Trie
	Log       *zerolog.Logger
	PC        net.PacketConn
	Done      chan bool
}

// Config for names
type Config struct {
	ListenerAddress  string
	CacheConfig      *cache.Config
	LoggerConfig     *LoggerConfig
	DNSClientNet     string
	DNSClientTimeout time.Duration
}

// LoggerConfig for creating the logger
type LoggerConfig struct {
	Filename   string
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Compress   bool
}

// serve responses to DNS requests
func (n *Names) serve() {
	n.Log.Print("PID: ", os.Getpid())
L:
	for {
		n.PC.SetDeadline(time.Now().Add(time.Duration(1) * time.Second))
		select {
		case <-n.Done:
			break L
		default:
			// read the query, shouldn't be more than 1024 bytes :grimace:
			buf := make([]byte, 1024)
			i, addr, err := n.PC.ReadFrom(buf)
			if err != nil {
				if e, ok := err.(net.Error); ok && e.Timeout() {
					continue
				}
				n.Log.Print(err)
				break L
			}
			go n.handleUDP(buf[:i], n.PC, addr)
		}
	}
	n.Log.Print("loop closed")
}

func makeLogger(config *LoggerConfig) *zerolog.Logger {
	file := &lumberjack.Logger{
		Filename:   config.Filename,
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   config.Compress,
	}
	multi := io.MultiWriter(file, os.Stdout)
	wr := diode.NewWriter(multi, 1000, 10*time.Millisecond, func(missed int) {
		fmt.Printf("logger dropped %d messages", missed)
	})
	log.Logger = log.Output(wr)
	return &log.Logger
}

func (n *Names) refreshCacheFunc(cache *cache.Cache) {
	for domain, element := range cache.Elements {
		resp := n.resolveUpstream(element.Request)
		n.cache.Set(domain, resp)
	}
}

// CreateListener returns a UDP listener
func CreateListener(addr string) (net.PacketConn, error) {
	return net.ListenPacket("udp", addr)
}

// New Names instance
func New(config *Config) (*Names, error) {
	n := &Names{
		dnsClient: dns.Client{
			Net:     config.DNSClientNet,
			Timeout: config.DNSClientTimeout,
		},
		Log:  makeLogger(config.LoggerConfig),
		tree: trie.NewTrie(),
	}
	var err error
	config.CacheConfig.RefreshFunc = n.refreshCacheFunc
	n.cache, err = cache.New(*config.CacheConfig)
	if err != nil {
		return n, errors.Wrap(err, "failed to setup cache")
	}
	// update the blacklists
	if n.tree, err = loadLists(n.Log, viper.GetBool("fetch-lists")); err != nil {
		return n, errors.Wrap(err, "failed to fetch and update blacklist")
	}
	// create the listener
	n.PC, err = CreateListener(config.ListenerAddress)
	if err != nil {
		return n, errors.Wrap(err, "failed to create listener")
	}
	n.Done = make(chan (bool))
	n.Log.Print("serving on ", config.ListenerAddress)
	return n, nil
}

// WaitForSignals blocks until interrupted
func waitForSignals() {
	var stopChan = make(chan os.Signal, 2)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-stopChan // wait for SIGINT
}

// Run the server
func (n *Names) Run() {
	go n.serve()
	waitForSignals()
	n.PC.Close()
}

func (n *Names) packAndWrite(msg *dns.Msg, pc net.PacketConn, addr net.Addr) error {
	data, err := msg.Pack()
	if err != nil {
		n.Log.Print("msg pack error", err)
		return err
	}
	if _, err := pc.WriteTo(data, addr); err != nil {
		n.Log.Print("msg write error", err)
		return err
	}
	return nil
}

func (n *Names) refreshCache(msg *dns.Msg) {
	n.cache.Set(msg.Question[0].Name, n.resolveUpstream(msg))
}

func (n *Names) handleUDP(buf []byte, pc net.PacketConn, addr net.Addr) {
	msg := new(dns.Msg)
	var err error
	if err := msg.Unpack(buf); err != nil {
		n.Log.Error().Err(err).Msg("failed to unpack request")
		return
	}

	if msg.Question[0].Name == "local." {
		n.Log.Print(msg.Question[0].Name)
		RR, err := dns.NewRR(fmt.Sprintf("%s 3600 IN A 127.0.0.1", msg.Question[0].Name))
		if err != nil {
			n.Log.Error().Err(err).Msg("failed to create local. response")
			return
		}
		msg.Answer = append(msg.Answer, RR)
		if err = n.packAndWrite(msg, pc, addr); err != nil {
			n.Log.Error().Err(err)
			return
		}
		return
	}

	if element, cacheHit := n.cache.Get(msg.Question[0].Name); cacheHit {
		msg.Answer = element.Value
		n.Log.Debug().Msgf("cache hit: %s", msg.Question[0].Name)
		if err = n.packAndWrite(msg, pc, addr); err != nil {
			n.Log.Error().Err(err)
			return
		}
		// Let's update the cache with the latest resolution
		if element.Refresh {
			go n.refreshCache(msg)
		}
		return
	}

	if n.isBlacklisted(msg.Question[0].Name) {
		n.Log.Debug().Msgf("%s did hit the blacklist", msg.Question[0].Name)
		RR, err := dns.NewRR(fmt.Sprintf("%s 3600 IN A 127.0.0.1", msg.Question[0].Name))
		if err != nil {
			n.Log.Error().Err(err).Msg("faile to create blacklist response")
			return
		}
		msg.Answer = append(msg.Answer, RR)
		element := cache.Element{Value: msg.Answer, Refresh: false, Request: msg}
		n.cache.Set(msg.Question[0].Name, element)
		if err = n.packAndWrite(msg, pc, addr); err != nil {
			n.Log.Error().Err(err).Msg("failed to send bl response")
			return
		}
		return
	}

	element := n.resolveUpstream(msg)
	element.Refresh = true
	n.cache.Set(msg.Question[0].Name, element)

	msg.Answer = element.Value
	n.packAndWrite(msg, pc, addr)
}
