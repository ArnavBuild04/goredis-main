package server

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/newpath/goredis/internal/command"
	"github.com/newpath/goredis/internal/pubsub"
	"github.com/newpath/goredis/internal/resp"
)

// subscriberModeAllowed is the small command set a connection may still
// issue once it has an active subscription — mirroring classic RESP2
// Redis, which refuses arbitrary commands on a subscribed connection
// because a push message and a command reply would otherwise be
// indistinguishable on the wire.
var subscriberModeAllowed = map[string]bool{
	"SUBSCRIBE":   true,
	"UNSUBSCRIBE": true,
	"PING":        true,
}

// handleConn owns one client connection end to end: parsing requests,
// dispatching them, forwarding any pub/sub messages the connection has
// subscribed to, and cleaning up on disconnect.
func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	reader := resp.NewReader(conn)
	writer := resp.NewWriter(conn)
	var writeMu sync.Mutex
	safeWrite := func(v resp.Value) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return writer.Write(v)
	}

	ctx := command.NewContext(s.registry, s.hub)
	if s.aof != nil {
		ctx.OnWrite = func(args []resp.Value) {
			if err := s.aof.Append(args); err != nil {
				s.log.Error("AOF append failed", "error", err)
			}
		}
	}
	defer ctx.Close()

	stopForwarder := make(chan struct{})
	defer close(stopForwarder)
	go forwardPubSub(ctx.Subscriber(), safeWrite, stopForwarder)

	for {
		req, err := reader.Read()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				s.log.Debug("connection read error", "error", err)
			}
			return
		}

		if req.Type != resp.TypeArray || len(req.Elems) == 0 {
			safeWrite(resp.Errorf("ERR invalid request, expected a non-empty array"))
			continue
		}

		name := strings.ToUpper(req.Elems[0].Str)
		args := req.Elems[1:]

		if ctx.InSubscriberMode() && !subscriberModeAllowed[name] {
			safeWrite(resp.Errorf("ERR Can't execute '%s': only (UN)SUBSCRIBE / PING are allowed in subscriber mode", strings.ToLower(name)))
			continue
		}

		result := s.registry.Dispatch(s.store, ctx, name, args)
		if err := safeWrite(result); err != nil {
			s.log.Debug("connection write error", "error", err)
			return
		}
	}
}

// forwardPubSub relays published messages to this connection's client for
// as long as the connection is open, independently of the request/response
// loop above — a message can arrive at any time, not just between requests.
func forwardPubSub(sub *pubsub.Subscriber, write func(resp.Value) error, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case msg := <-sub.Messages():
			frame := resp.Array(resp.Bulk("message"), resp.Bulk(msg.Channel), resp.Bulk(msg.Payload))
			if write(frame) != nil {
				return
			}
		}
	}
}
