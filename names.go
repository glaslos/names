package names

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/fvbock/trie"
	"github.com/miekg/dns"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Names main struct
type Names struct {
	cache     Cache
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
func (n *Names) Serve() {
	n.Log.Print(os.Getpid())
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
	n.Log.Print("Loop closed")
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

// New Names instance
func New() *Names {
	n := &Names{
		dnsClient: dns.Client{
			Net:     "tcp-tls",
			Timeout: time.Duration(10) * time.Second,
		},
		cache: InitCache(100 * 1000000000),
		Log:   makeLogger(),
		tree:  trie.NewTrie(),
	}
	// update the blacklists
	fetchLists(n.Log, n.tree)
	// create the listener
	pc, err := CreateOrImportListener("127.0.0.1:53")
	if err != nil {
		n.Log.Fatal().Err(err)
	}

	n.Log.Print("serving on 127.0.0.1:53")
	n.server = &Server{PC: pc, Done: make(chan (bool))}
	go n.Serve()
	return n
}

// Run the server
func (n *Names) Run() {
	defer n.server.PC.Close()
	err := WaitForSignals("127.0.0.1:53", n.server.PC, n.server)
	if err != nil {
		n.Log.Printf("Exiting: %v\n", err)
		return
	}
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

	if n.tree.Has(strings.Trim(msg.Question[0].Name, ".")) {
		n.Log.Printf("%s did hit the blacklist", msg.Question[0].Name)
		RR, err := dns.NewRR(fmt.Sprintf("%s 3600 IN A 127.0.0.1", msg.Question[0].Name))
		if err != nil {
			n.Log.Print("udp parse msg error", err)
			return
		}
		msg.Answer = append(msg.Answer, RR)
		element := Element{Value: msg.Answer, Refresh: false}
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
	n.Log.Print(msg.Question[0].Name, element.Resolver)

	n.packAndWrite(msg, pc, addr)
}
