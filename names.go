package names

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/glaslos/names/cache"
	"github.com/glaslos/names/lists"

	"github.com/glaslos/trie"
	"github.com/miekg/dns"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
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
			go func() {
				if err := n.handleUDP(buf[:i], n.PC, addr); err != nil {
					n.Log.Error().Err(err).Msg("failed to unpack request")
				}
			}()
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
		resp, err := n.resolveUpstream(element.Request)
		if err != nil {
			// handle error here?
			return
		}
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
	// update the blocklists
	if n.tree, err = lists.Load(); err != nil {
		return n, errors.Wrap(err, "failed to load blocklist")
	}
	if fetchList := viper.GetStringSlice("fetch-lists"); len(fetchList) > 0 {
		if err := lists.PopulateCache(n.tree, fetchList, n.Log); err != nil {
			return n, errors.Wrap(err, "failed to fetch and update blocklists")
		}
	}
	if err := lists.Dump(n.tree); err != nil {
		return n, errors.Wrap(err, "failed to dump block list to file")
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

func (n *Names) isBlocklisted(name string) bool {
	return n.tree.Has(lists.ReverseString(strings.Trim(name, ".")))
}

func (n *Names) packAndWrite(msg *dns.Msg, pc net.PacketConn, addr net.Addr) error {
	data, err := msg.Pack()
	if err != nil {
		return fmt.Errorf("failed to msg pack: %w", err)
	}
	if _, err := pc.WriteTo(data, addr); err != nil {
		return fmt.Errorf("failed to write msg: %w", err)
	}
	return nil
}

func validate(msg *dns.Msg) error {
	if len(msg.Question) == 0 {
		return errors.New("no question")
	}
	if msg.Question[0].Name == "" {
		return errors.New("missing name")
	}
	return nil
}

func (n *Names) handleUDP(buf []byte, pc net.PacketConn, addr net.Addr) error {
	msg := new(dns.Msg)
	if err := msg.Unpack(buf); err != nil {
		return fmt.Errorf("failed to unpack request: %w", err)
	}

	if err := validate(msg); err != nil {
		return err
	}

	if msg.Question[0].Name == "local." {
		n.Log.Print(msg.Question[0].Name)
		RR, err := dns.NewRR(fmt.Sprintf("%s 3600 IN A 127.0.0.1", msg.Question[0].Name))
		if err != nil {
			return fmt.Errorf("failed to create local. response: %w", err)
		}
		msg.Answer = append(msg.Answer, RR)
		if err = n.packAndWrite(msg, pc, addr); err != nil {
			return err
		}
		return nil
	}

	if element, cacheHit := n.cache.Get(msg.Question[0].Name); cacheHit {
		msg.Answer = element.Value
		n.Log.Debug().Msgf("cache hit: %s", msg.Question[0].Name)
		if err := n.packAndWrite(msg, pc, addr); err != nil {
			return err
		}
		// Let's update the cache with the latest resolution
		if element.Refresh {
			go func() {
				element, err := n.resolveUpstream(msg)
				if err != nil {
					// handle error?
					return
				}
				n.cache.Set(msg.Question[0].Name, element)
			}()
		}
		return nil
	}

	if n.isBlocklisted(msg.Question[0].Name) {
		n.Log.Debug().Msgf("%s did hit the blocklist", msg.Question[0].Name)
		RR, err := dns.NewRR(fmt.Sprintf("%s 3600 IN A 127.0.0.1", msg.Question[0].Name))
		if err != nil {
			return errors.New("faile to create blocklist response")
		}
		msg.Answer = append(msg.Answer, RR)
		go func() {
			// set cache since it was a cache miss
			element := cache.Element{Value: msg.Answer, Refresh: false, Request: msg}
			n.cache.Set(msg.Question[0].Name, element)

		}()
		if err = n.packAndWrite(msg, pc, addr); err != nil {
			return errors.New("failed to send bl response")
		}
		return nil
	}

	element, err := n.resolveUpstream(msg)
	if err != nil {
		return err
	}
	go func() {
		element.Refresh = true
		n.cache.Set(msg.Question[0].Name, element)
	}()

	msg.Answer = element.Value
	return n.packAndWrite(msg, pc, addr)
}
