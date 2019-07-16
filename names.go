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
	server    *Server
}

// Server is a UDP DNS server
type Server struct {
	PC   net.PacketConn
	Done chan bool
}

func (s *Server) stop() error {
	s.Done <- true
	return nil
}

// Serve responses to DNS requests
func (n *Names) serve() {
	n.Log.Print("PID: ", os.Getpid())
L:
	for {
		n.server.PC.SetDeadline(time.Now().Add(time.Duration(1) * time.Second))
		select {
		case <-n.server.Done:
			break L
		default:
			// read the query, shouldn't be more than 1024 bytes :grimace:
			buf := make([]byte, 1024)
			i, addr, err := n.server.PC.ReadFrom(buf)
			if err != nil {
				if e, ok := err.(net.Error); ok && e.Timeout() {
					continue
				}
				n.Log.Print(err)
				break L
			}
			go n.handleUDP(buf[:i], n.server.PC, addr)
		}
	}
	n.Log.Print("loop closed")
}

func makeLogger() *zerolog.Logger {
	file := &lumberjack.Logger{
		Filename:   "./names.log",
		MaxSize:    500, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   //days
		Compress:   true, // disabled by default
	}
	multi := io.MultiWriter(file, os.Stdout)
	wr := diode.NewWriter(multi, 1000, 10*time.Millisecond, func(missed int) {
		fmt.Printf("Logger Dropped %d messages", missed)
	})
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: wr})
	return &log.Logger
}

func (n *Names) refreshCacheFunc(cache *cache.Cache) {
	for domain, element := range cache.Elements {
		resp := n.resolveUpstream(element.Request)
		n.cache.Set(domain, resp)
	}
}

// New Names instance
func New() (*Names, error) {
	n := &Names{
		dnsClient: dns.Client{
			Net:     "tcp-tls",
			Timeout: time.Duration(10) * time.Second,
		},
		Log:  makeLogger(),
		tree: trie.NewTrie(),
	}
	var err error
	n.cache, err = cache.New(cache.Config{
		ExpirationTime:  (100 * time.Millisecond).Nanoseconds(),
		RefreshInterval: 10 * time.Second,
		RefreshFunc:     n.refreshCacheFunc,
		Persist:         false,
	})
	if err != nil {
		return n, errors.Wrap(err, "failed to setup cache")
	}
	// update the blacklists
	if n.tree, err = fetchLists(n.Log, n.tree); err != nil {
		return n, errors.Wrap(err, "failed to fetch and update blacklist")
	}
	// create the listener
	pc, err := CreateListener("127.0.0.1:53")
	if err != nil {
		return n, errors.Wrap(err, "failed to create listener")
	}

	n.Log.Print("serving on 127.0.0.1:53")
	n.server = &Server{PC: pc, Done: make(chan (bool))}
	return n, nil
}

// WaitForSignals blocks until interrupted
func WaitForSignals() {
	var stopChan = make(chan os.Signal, 2)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-stopChan // wait for SIGINT
}

// CreateListener returns a UDP listener
func CreateListener(addr string) (net.PacketConn, error) {
	return net.ListenPacket("udp", addr)
}

// Run the server
func (n *Names) Run() {
	go n.serve()
	WaitForSignals()
	n.server.PC.Close()
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
		n.Log.Error().Err(err)
		return
	}

	if msg.Question[0].Name == "local." {
		n.Log.Print(msg.Question[0].Name)
		RR, err := dns.NewRR(fmt.Sprintf("%s 3600 IN A 127.0.0.1", msg.Question[0].Name))
		if err != nil {
			n.Log.Error().Err(err)
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
		n.Log.Printf("%s did hit the cache", msg.Question[0].Name)
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
		n.Log.Printf("%s did hit the blacklist", msg.Question[0].Name)
		RR, err := dns.NewRR(fmt.Sprintf("%s 3600 IN A 127.0.0.1", msg.Question[0].Name))
		if err != nil {
			n.Log.Print("udp parse msg error", err)
			return
		}
		msg.Answer = append(msg.Answer, RR)
		element := cache.Element{Value: msg.Answer, Refresh: false, Request: msg}
		n.cache.Set(msg.Question[0].Name, element)
		if err = n.packAndWrite(msg, pc, addr); err != nil {
			n.Log.Error().Err(err)
			return
		}
		return
	}

	element := n.resolveUpstream(msg)
	element.Refresh = true
	n.cache.Set(msg.Question[0].Name, element)
	//n.Log.Print(msg.Question[0].Name, " ", msg.Question[0].Qtype, " ", element.Value[0].String(), " ", element.Resolver)

	n.packAndWrite(msg, pc, addr)
}
