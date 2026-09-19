// Package server owns the TCP listener and connection lifecycle: accepting
// clients, wiring each one to the command registry, and coordinating
// graceful shutdown. It knows nothing about individual Redis commands —
// that's the command package's job.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/newpath/goredis/internal/aof"
	"github.com/newpath/goredis/internal/command"
	"github.com/newpath/goredis/internal/pubsub"
	"github.com/newpath/goredis/internal/resp"
	"github.com/newpath/goredis/internal/store"
)

// Config holds the server's runtime settings.
type Config struct {
	Addr    string // e.g. "localhost:6379"
	AOFPath string // empty disables persistence
}

// Server accepts connections and dispatches their commands.
type Server struct {
	cfg      Config
	log      *slog.Logger
	store    *store.Store
	registry *command.Registry
	hub      *pubsub.Hub
	aof      *aof.AOF

	ln      net.Listener
	closing chan struct{}
	closeOn sync.Once
	wg      sync.WaitGroup

	connsMu sync.Mutex
	conns   map[net.Conn]struct{}
}

// New builds a Server: opens the AOF (if configured) and replays it to
// reconstruct the keyspace before accepting any client connections.
func New(cfg Config, log *slog.Logger) (*Server, error) {
	s := &Server{
		cfg:      cfg,
		log:      log,
		store:    store.New(),
		registry: command.NewRegistry(),
		hub:      pubsub.NewHub(),
		closing:  make(chan struct{}),
		conns:    make(map[net.Conn]struct{}),
	}

	if cfg.AOFPath != "" {
		a, err := aof.Open(cfg.AOFPath)
		if err != nil {
			return nil, err
		}
		s.aof = a
		if err := s.replay(); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// replay reconstructs the keyspace from the AOF. It runs before the server
// accepts connections, so it uses a throwaway Context with no AOF hook —
// otherwise replaying the log would just re-append everything it reads.
func (s *Server) replay() error {
	ctx := command.NewContext(s.registry, s.hub)
	n := 0
	err := s.aof.Replay(func(args []resp.Value) error {
		if len(args) == 0 {
			return nil
		}
		name := args[0].Str
		result := s.registry.Dispatch(s.store, ctx, name, args[1:])
		if result.Type == resp.TypeError {
			s.log.Warn("AOF replay: command failed", "command", name, "error", result.Str)
		}
		n++
		return nil
	})
	if err != nil {
		return err
	}
	if n > 0 {
		s.log.Info("AOF replay complete", "commands", n)
	}
	return nil
}

// Run listens on cfg.Addr and serves connections until ctx is canceled,
// then drains active connections (up to shutdownGrace) before returning.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	s.ln = ln
	s.log.Info("listening", "addr", ln.Addr().String())

	serveErr := make(chan error, 1)
	go func() { serveErr <- s.acceptLoop() }()

	select {
	case <-ctx.Done():
		return s.shutdown()
	case err := <-serveErr:
		return err
	}
}

func (s *Server) acceptLoop() error {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.closing:
				return nil
			default:
				return err
			}
		}
		s.trackConn(conn)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer s.untrackConn(conn)
			s.handleConn(conn)
		}()
	}
}

func (s *Server) trackConn(c net.Conn) {
	s.connsMu.Lock()
	s.conns[c] = struct{}{}
	s.connsMu.Unlock()
}

func (s *Server) untrackConn(c net.Conn) {
	s.connsMu.Lock()
	delete(s.conns, c)
	s.connsMu.Unlock()
}

const shutdownGrace = 5 * time.Second

func (s *Server) shutdown() error {
	s.log.Info("shutting down")
	s.closeOn.Do(func() { close(s.closing) })
	s.ln.Close()

	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(shutdownGrace):
		s.log.Warn("shutdown grace period elapsed; force-closing remaining connections")
		s.connsMu.Lock()
		for c := range s.conns {
			c.Close()
		}
		s.connsMu.Unlock()
		<-done // Close() unblocks each connection's pending Read, so this returns promptly.
	}

	s.store.Close()
	if s.aof != nil {
		if err := s.aof.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			return err
		}
	}
	return nil
}
