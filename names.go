package names

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/glaslos/names/cache"
	"github.com/glaslos/names/lists"

	"github.com/glaslos/trie"
	"github.com/phuslu/fastdns"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Upstream struct {
	addr   string
	client *fastdns.Client
}

// Names main struct
type Names struct {
	ctx          context.Context
	cache        *cache.Cache
	dnsUpstreams []*Upstream
	tree         *trie.Trie
	Log          *zerolog.Logger
	PC           net.PacketConn
	Done         chan bool
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
					n.Log.Error().Err(err).Msg("failed to handle request")
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

func (n *Names) dummyRefreshCacheFunc(cache *cache.Cache) {}

func (n *Names) refreshCacheFunc(cache *cache.Cache) {
	for domain, element := range cache.Elements {
		req := fastdns.AcquireMessage()
		defer fastdns.ReleaseMessage(req)
		if err := fastdns.ParseMessage(req, element.Request, true); err != nil {
			n.Log.Debug().Err(err)
			return
		}
		resp, err := n.resolveUpstream(req)
		if err != nil {
			n.Log.Debug().Err(err)
			return
		}
		n.cache.Set(domain, resp)
	}
}

// CreateListener returns a UDP listener
func CreateListener(addr string) (net.PacketConn, error) {
	return net.ListenPacket("udp", addr)
}

func (n *Names) makeUpstreams() error {
	for _, upstream := range viper.GetStringSlice("upstreams") {
		server, sport, err := net.SplitHostPort(upstream)
		if err != nil {
			return err
		}
		port, err := strconv.Atoi(sport)
		if err != nil {
			return err
		}
		client, err := newClient(server, int16(port))
		if err != nil {
			return err
		}
		n.dnsUpstreams = append(n.dnsUpstreams, &Upstream{
			addr:   upstream,
			client: client,
		})
	}
	return nil
}

// New Names instance
func New(ctx context.Context, config *Config) (*Names, error) {
	n := &Names{
		ctx:  ctx,
		Log:  makeLogger(config.LoggerConfig),
		tree: trie.NewTrie(),
	}
	if err := n.makeUpstreams(); err != nil {
		return nil, err
	}

	switch config.CacheConfig.RefreshCache {
	case true:
		config.CacheConfig.RefreshFunc = n.refreshCacheFunc
	default:
		config.CacheConfig.RefreshFunc = n.dummyRefreshCacheFunc
	}

	var err error
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

func (n *Names) write(data []byte, pc net.PacketConn, addr net.Addr) error {
	if _, err := pc.WriteTo(data, addr); err != nil {
		return fmt.Errorf("failed to write msg: %w", err)
	}
	return nil
}

func validateFast(msg *fastdns.Message) error {
	if len(msg.Domain) == 0 {
		return errors.New("no question")
	}
	return nil
}

func makeResponse(resp *fastdns.Message, addr string) (*fastdns.Message, error) {
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return resp, err
	}
	ips := []netip.Addr{ip}
	resp.SetResponseHeader(fastdns.RcodeNoError, uint16(len(ips)))
	resp.Raw = fastdns.AppendHOSTRecord(resp.Raw, resp, 300, ips)
	return resp, nil
}

func (n *Names) handleUDP(buf []byte, pc net.PacketConn, addr net.Addr) error {
	req := fastdns.AcquireMessage()
	defer fastdns.ReleaseMessage(req)

	if err := fastdns.ParseMessage(req, buf, true); err != nil {
		return err
	}

	if err := validateFast(req); err != nil {
		return err
	}

	n.Log.Debug().Msgf("lookup: %v", string(req.Domain))

	// local lookup
	if strings.TrimSpace(string(req.Domain)) == "local" {
		resp, err := makeResponse(req, "127.0.0.1")
		if err != nil {
			return err
		}
		if err := n.write(resp.Raw, pc, addr); err != nil {
			return err
		}
		return nil
	}

	// cache hit?
	if element, cacheHit := n.cache.Get(string(req.Domain)); cacheHit {
		n.Log.Debug().Msg("cache hit")
		resp, err := makeResponse(req, element.Value)
		if err != nil {
			return err
		}
		if err := n.write(resp.Raw, pc, addr); err != nil {
			return err
		}

		// Let's update the cache with the latest resolution
		if element.Refresh {
			go func() {
				element, err := n.resolveUpstream(req)
				if err != nil {
					// handle error?
					return
				}
				n.Log.Debug().Msgf("Refreshed: %s", string(req.Domain))
				n.cache.Set(string(req.Domain), element)
			}()
		}
		return nil
	}

	// block list?
	if n.isBlocklisted(string(req.Domain)) {
		n.Log.Debug().Msgf("%s did hit the blocklist", string(req.Domain))
		resp, err := makeResponse(req, "127.0.0.1")
		if err != nil {
			return err
		}
		if err := n.write(resp.Raw, pc, addr); err != nil {
			return err
		}
		go func() {
			// set cache since it was a cache miss
			element := cache.Element{Value: "127.0.0.1", Refresh: false, Request: buf}
			n.cache.Set(string(req.Domain), element)

		}()
		return nil
	}

	// regular resolve
	element, err := n.resolveUpstream(req)
	if err != nil {
		return err
	}

	resp, err := makeResponse(req, element.Value)
	if err != nil {
		return err
	}

	go func() {
		element.Refresh = true
		n.cache.Set(string(req.Domain), element)
	}()

	return n.write(resp.Raw, pc, addr)
}
