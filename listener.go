package names

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/rs/zerolog/log"
)

// listener inspired by https://gravitational.com/blog/golang-ssh-bastion-graceful-restarts/
type listener struct {
	Addr     string `json:"addr"`
	FD       int    `json:"fd"`
	Filename string `json:"filename"`
}

func importListener(addr string) (net.PacketConn, error) {
	// Extract the encoded listener metadata from the environment.
	listenerEnv := os.Getenv("LISTENER")
	if listenerEnv == "" {
		return nil, errors.New("unable to find LISTENER environment variable")
	}

	// Unmarshal the listener metadata.
	var l listener
	err := json.Unmarshal([]byte(listenerEnv), &l)
	if err != nil {
		return nil, err
	}
	if l.Addr != addr {
		return nil, fmt.Errorf("unable to find listener for %v", addr)
	}

	// The file has already been passed to this process, extract the file
	// descriptor and name from the metadata to rebuild/find the *os.File for
	// the listener.
	listenerFile := os.NewFile(uintptr(l.FD), l.Filename)
	if listenerFile == nil {
		return nil, fmt.Errorf("unable to create listener file: %v", err)
	}
	defer listenerFile.Close()

	// Create a net.Listener from the *os.File.
	ln, err := net.FilePacketConn(listenerFile)
	if err != nil {
		return nil, err
	}

	return ln, nil
}

func createListener(addr string) (net.PacketConn, error) {
	return net.ListenPacket("udp", addr)
}

// CreateOrImportListener returns a connection for an address
func CreateOrImportListener(addr string) (net.PacketConn, error) {
	// Try and import a listener for addr. If it's found, use it.
	ln, err := importListener(addr)
	if err == nil {
		log.Printf("imported listener file descriptor for %v", addr)
		return ln, nil
	}

	// No listener was imported, that means this process has to create one.
	ln, err = createListener(addr)
	if err != nil {
		return nil, err
	}

	log.Printf("created listener file descriptor for %v", addr)
	return ln, nil
}

func getListenerFile(conn net.PacketConn) (*os.File, error) {
	switch t := conn.(type) {
	case *net.UDPConn:
		return t.File()
	}
	return nil, fmt.Errorf("unsupported listener: %T", conn)
}

func forkChild(addr string, conn net.PacketConn) (*os.Process, error) {
	// Get the file descriptor for the listener and marshal the metadata to pass
	// to the child in the environment.
	lnFile, err := getListenerFile(conn)
	if err != nil {
		return nil, err
	}
	defer lnFile.Close()
	l := listener{
		Addr:     addr,
		FD:       3,
		Filename: lnFile.Name(),
	}
	listenerEnv, err := json.Marshal(l)
	if err != nil {
		return nil, err
	}

	// Pass stdin, stdout, and stderr along with the listener to the child.
	files := []*os.File{
		os.Stdin,
		os.Stdout,
		os.Stderr,
		lnFile,
	}

	// Get current environment and add in the listener to it.
	environment := append(os.Environ(), "LISTENER="+string(listenerEnv))

	// Get current process name and directory.
	execName, err := os.Executable()
	if err != nil {
		return nil, err
	}
	execDir := filepath.Dir(execName)

	// Spawn child process.
	p, err := os.StartProcess(execName, []string{execName}, &os.ProcAttr{
		Dir:   execDir,
		Env:   environment,
		Files: files,
		Sys:   &syscall.SysProcAttr{},
	})
	if err != nil {
		return nil, err
	}

	return p, nil
}

// WaitForSignals handles signals
func WaitForSignals(addr string, conn net.PacketConn, server *Server) error {
	signalCh := make(chan os.Signal, 1024)
	signal.Notify(signalCh, syscall.SIGHUP, syscall.SIGUSR2, syscall.SIGINT, syscall.SIGQUIT)
	for {
		select {
		case s := <-signalCh:
			log.Printf("%v signal received", s)
			switch s {
			case syscall.SIGHUP:
				// Fork a child process.
				p, err := forkChild(addr, conn)
				if err != nil {
					log.Printf("unable to fork child: %v", err)
					continue
				}
				log.Printf("forked child %v", p.Pid)

				// Create a context that will expire in 5 seconds and use this as a
				// timeout to Shutdown.
				// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				// defer cancel()

				// Return any errors during shutdown.
				return server.stop()
			case syscall.SIGUSR2:
				// Fork a child process.
				p, err := forkChild(addr, conn)
				if err != nil {
					log.Printf("unable to fork child: %v", err)
					continue
				}

				// Print the PID of the forked process and keep waiting for more signals.
				log.Printf("forked child %v", p.Pid)
			case syscall.SIGINT, syscall.SIGQUIT:
				// Create a context that will expire in 5 seconds and use this as a
				// timeout to Shutdown.

				// Return any errors during shutdown.
				return server.stop()
			}
		}
	}
}
