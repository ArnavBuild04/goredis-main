// Package aof implements append-only-file persistence: every write command
// is marshaled as a RESP array and appended to a log file, which is replayed
// in full on startup to reconstruct the in-memory keyspace. It has no
// opinion on which commands count as writes — that's the command
// package's Spec.Write flag, kept in one place instead of duplicated here.
package aof

import (
	"bufio"
	"io"
	"os"
	"sync"
	"time"

	"github.com/newpath/goredis/internal/resp"
)

// AOF wraps the on-disk log file: a background goroutine fsyncs it once a
// second so a crash loses at most ~1s of writes, matching Redis's default
// `appendfsync everysec` durability/throughput trade-off.
type AOF struct {
	file *os.File

	mu       sync.Mutex
	stopSync chan struct{}
	syncOnce sync.Once
}

// Open opens (creating if necessary) the AOF file at path and starts its
// background fsync loop.
func Open(path string) (*AOF, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o666)
	if err != nil {
		return nil, err
	}

	a := &AOF{file: f, stopSync: make(chan struct{})}
	go a.syncLoop(time.Second)
	return a, nil
}

func (a *AOF) syncLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-a.stopSync:
			return
		case <-ticker.C:
			a.mu.Lock()
			a.file.Sync()
			a.mu.Unlock()
		}
	}
}

// Append writes one command frame, encoded exactly as it would be sent
// over the wire, to the log.
func (a *AOF) Append(args []resp.Value) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, err := a.file.Write(resp.Array(args...).Marshal())
	return err
}

// Replay reads every frame in the log from the beginning and invokes fn
// with each one's arguments (command name plus its own arguments, as a
// single flat array), in the order they were written.
func (a *AOF) Replay(fn func(args []resp.Value) error) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, err := a.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	r := resp.NewReader(bufio.NewReader(a.file))

	for {
		v, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if err := fn(v.Elems); err != nil {
			return err
		}
	}

	_, err := a.file.Seek(0, io.SeekEnd)
	return err
}

// Close stops the background sync loop and closes the underlying file.
func (a *AOF) Close() error {
	a.syncOnce.Do(func() { close(a.stopSync) })
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.file.Close()
}
