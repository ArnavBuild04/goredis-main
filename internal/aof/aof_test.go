package aof

import (
	"path/filepath"
	"testing"

	"github.com/newpath/goredis/internal/resp"
)

func TestAppendAndReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.aof")

	a, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	commands := [][]resp.Value{
		{resp.Bulk("SET"), resp.Bulk("k1"), resp.Bulk("v1")},
		{resp.Bulk("SET"), resp.Bulk("k2"), resp.Bulk("v2")},
		{resp.Bulk("DEL"), resp.Bulk("k1")},
	}
	for _, c := range commands {
		if err := a.Append(c); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	a2, err := Open(path)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	defer a2.Close()

	var replayed [][]string
	err = a2.Replay(func(args []resp.Value) error {
		row := make([]string, len(args))
		for i, v := range args {
			row[i] = v.Str
		}
		replayed = append(replayed, row)
		return nil
	})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if len(replayed) != 3 {
		t.Fatalf("replayed %d commands, want 3: %v", len(replayed), replayed)
	}
	if replayed[0][0] != "SET" || replayed[0][1] != "k1" || replayed[0][2] != "v1" {
		t.Fatalf("replayed[0] = %v", replayed[0])
	}
	if replayed[2][0] != "DEL" || replayed[2][1] != "k1" {
		t.Fatalf("replayed[2] = %v", replayed[2])
	}
}

func TestAppendAfterReplayContinuesAtEnd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.aof")
	a, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()

	a.Append([]resp.Value{resp.Bulk("SET"), resp.Bulk("a"), resp.Bulk("1")})

	var n int
	a.Replay(func(args []resp.Value) error { n++; return nil })
	if n != 1 {
		t.Fatalf("first replay saw %d commands, want 1", n)
	}

	a.Append([]resp.Value{resp.Bulk("SET"), resp.Bulk("b"), resp.Bulk("2")})

	n = 0
	a.Replay(func(args []resp.Value) error { n++; return nil })
	if n != 2 {
		t.Fatalf("second replay saw %d commands, want 2 (append after replay must land after existing data)", n)
	}
}
