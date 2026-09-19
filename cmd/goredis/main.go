// Command goredis runs a Redis-protocol-compatible server. Connect to it
// with any RESP2 client, including redis-cli.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/newpath/goredis/internal/server"
)

func main() {
	host := flag.String("host", "0.0.0.0", "interface to listen on")
	port := flag.String("port", "6379", "port to listen on")
	aofPath := flag.String("aof", "db.aof", "append-only-file path; empty disables persistence")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	srv, err := server.New(server.Config{
		Addr:    net.JoinHostPort(*host, *port),
		AOFPath: *aofPath,
	}, log)
	if err != nil {
		log.Error("failed to start", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := srv.Run(ctx); err != nil {
		log.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}
