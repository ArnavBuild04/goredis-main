package server

import (
	"bufio"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testClient is a minimal RESP client good enough to drive integration
// tests without depending on the command/resp packages directly — it
// exercises the server the way a real client (or redis-cli) would, over
// the actual wire format.
type testClient struct {
	t    *testing.T
	conn net.Conn
	r    *bufio.Reader
}

func dial(t *testing.T, addr string) *testClient {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return &testClient{t: t, conn: conn, r: bufio.NewReader(conn)}
}

func (c *testClient) send(args ...string) {
	c.t.Helper()
	var b strings.Builder
	b.WriteString("*")
	b.WriteString(itoa(len(args)))
	b.WriteString("\r\n")
	for _, a := range args {
		b.WriteString("$")
		b.WriteString(itoa(len(a)))
		b.WriteString("\r\n")
		b.WriteString(a)
		b.WriteString("\r\n")
	}
	if _, err := c.conn.Write([]byte(b.String())); err != nil {
		c.t.Fatalf("Write: %v", err)
	}
}

func (c *testClient) readLine() string {
	c.t.Helper()
	line, err := c.r.ReadString('\n')
	if err != nil {
		c.t.Fatalf("readLine: %v", err)
	}
	return strings.TrimRight(line, "\r\n")
}

// readReply reads one reply frame and returns it in RESP's own wire prefix
// form (e.g. "+OK", ":1", "$-1"), plus any bulk payload on the next line —
// enough for equality checks in tests without building a full parser.
func (c *testClient) readReply() string {
	c.t.Helper()
	line := c.readLine()
	if strings.HasPrefix(line, "$") && line != "$-1" {
		payload := c.readLine()
		return line + "\r\n" + payload
	}
	return line
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func startTestServer(t *testing.T) string {
	t.Helper()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	s, err := New(Config{Addr: "127.0.0.1:0"}, log)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	s.ln = ln
	addr := ln.Addr().String()

	done := make(chan struct{})
	go func() {
		s.acceptLoop()
		close(done)
	}()

	t.Cleanup(func() {
		s.shutdown()
		<-done
	})

	return addr
}

func TestServerPingAndSetGet(t *testing.T) {
	addr := startTestServer(t)
	c := dial(t, addr)

	c.send("PING")
	if got := c.readReply(); got != "+PONG" {
		t.Fatalf("PING = %q, want +PONG", got)
	}

	c.send("SET", "foo", "bar")
	if got := c.readReply(); got != "+OK" {
		t.Fatalf("SET = %q, want +OK", got)
	}

	c.send("GET", "foo")
	if got := c.readReply(); got != "$3\r\nbar" {
		t.Fatalf("GET = %q, want $3\\r\\nbar", got)
	}
}

func TestServerPersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	aofPath := filepath.Join(dir, "db.aof")
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	func() {
		s, err := New(Config{Addr: "127.0.0.1:0", AOFPath: aofPath}, log)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Listen: %v", err)
		}
		s.ln = ln

		done := make(chan struct{})
		go func() { s.acceptLoop(); close(done) }()

		c := dial(t, ln.Addr().String())
		c.send("SET", "persisted", "yes")
		if got := c.readReply(); got != "+OK" {
			t.Fatalf("SET = %q", got)
		}
		c.conn.Close() // let the connection drain promptly instead of waiting out shutdown's grace period

		s.shutdown() // flushes and closes the AOF before the next instance opens it
		<-done
	}()

	// Second instance, same AOF path: SET above must have been replayed.
	s2, err := New(Config{Addr: "127.0.0.1:0", AOFPath: aofPath}, log)
	if err != nil {
		t.Fatalf("New (restart): %v", err)
	}
	v, found, err := s2.store.Get("persisted")
	if err != nil || !found || v != "yes" {
		t.Fatalf("after restart, Get(persisted) = %q, %v, %v; want yes, true, nil", v, found, err)
	}
	s2.store.Close()
}

func TestServerPubSub(t *testing.T) {
	addr := startTestServer(t)
	sub := dial(t, addr)
	pub := dial(t, addr)

	sub.send("SUBSCRIBE", "news")
	if got := sub.readReply(); got != "*3" {
		t.Fatalf("SUBSCRIBE header = %q, want *3", got)
	}
	// drain the rest of the subscribe confirmation frame (bulk "subscribe", bulk "news", int 1)
	sub.readReply()
	sub.readReply()
	sub.readReply()

	pub.send("PUBLISH", "news", "hello")
	if got := pub.readReply(); got != ":1" {
		t.Fatalf("PUBLISH = %q, want :1 (one subscriber)", got)
	}

	if got := sub.readReply(); got != "*3" {
		t.Fatalf("message header = %q, want *3", got)
	}
	sub.readReply() // "message"
	sub.readReply() // "news"
	if got := sub.readReply(); got != "$5\r\nhello" {
		t.Fatalf("message payload = %q, want $5\\r\\nhello", got)
	}
}
