package resp

import (
	"bytes"
	"reflect"
	"testing"
)

func TestMarshalRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   Value
	}{
		{"simple string", Str("OK")},
		{"error", Value{Type: TypeError, Str: "ERR bad thing"}},
		{"integer", Int(42)},
		{"negative integer", Int(-1)},
		{"bulk", Bulk("hello world")},
		{"empty bulk", Bulk("")},
		{"null bulk", NullBulk()},
		{"null array", NullArray()},
		{"empty array", Array()},
		{"nested array", Array(Bulk("SET"), Bulk("k"), Bulk("v"))},
		{"array of mixed", Array(Int(1), Bulk("x"), Str("PONG"))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wire := tc.in.Marshal()
			got, err := NewReader(bytes.NewReader(wire)).Read()
			if err != nil {
				t.Fatalf("Read() error = %v", err)
			}
			if !reflect.DeepEqual(normalize(tc.in), normalize(got)) {
				t.Fatalf("round-trip mismatch:\n  in:  %#v\n  out: %#v", tc.in, got)
			}
		})
	}
}

// normalize nils out a zero-length-but-non-nil Elems slice so Array() and a
// parsed empty array compare equal regardless of which one is nil.
func normalize(v Value) Value {
	if v.Type == TypeArray && len(v.Elems) == 0 {
		v.Elems = nil
	} else {
		for i := range v.Elems {
			v.Elems[i] = normalize(v.Elems[i])
		}
	}
	return v
}

func TestReadClientCommand(t *testing.T) {
	wire := "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"
	v, err := NewReader(bytes.NewReader([]byte(wire))).Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if v.Type != TypeArray || len(v.Elems) != 3 {
		t.Fatalf("got %#v, want 3-element array", v)
	}
	want := []string{"SET", "foo", "bar"}
	for i, w := range want {
		if v.Elems[i].Str != w {
			t.Errorf("elem %d = %q, want %q", i, v.Elems[i].Str, w)
		}
	}
}

func TestWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.Write(OK()); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if buf.String() != "+OK\r\n" {
		t.Fatalf("got %q, want %q", buf.String(), "+OK\r\n")
	}
}
