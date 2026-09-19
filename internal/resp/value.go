// Package resp implements the RESP2 (REdis Serialization Protocol) wire
// format: parsing bytes off a connection into Values, and marshaling Values
// back into the same wire format. See https://redis.io/docs/reference/protocol-spec/
package resp

import (
	"fmt"
	"strconv"
)

// Type is one of the five RESP2 frame markers.
type Type byte

const (
	TypeSimpleString Type = '+'
	TypeError        Type = '-'
	TypeInteger      Type = ':'
	TypeBulk         Type = '$'
	TypeArray        Type = '*'
)

// Value is a single RESP frame. Which fields are meaningful depends on Type:
// SimpleString/Error use Str, Integer uses Num, Bulk uses Str (Null when
// absent), Array uses Elems (Null when absent).
type Value struct {
	Type  Type
	Str   string
	Num   int64
	Elems []Value
	Null  bool

	// Multi marks Elems as a bundle of independent top-level frames rather
	// than a single RESP array — used by commands like SUBSCRIBE that reply
	// with one frame per argument. Writer.Write unpacks it accordingly.
	Multi bool
}

// MultiFrame bundles several Values so the Writer emits each as its own
// top-level frame instead of nesting them in one array.
func MultiFrame(vs ...Value) Value {
	return Value{Type: TypeArray, Elems: vs, Multi: true}
}

func Str(s string) Value { return Value{Type: TypeSimpleString, Str: s} }
func OK() Value          { return Str("OK") }

func Errorf(format string, a ...any) Value {
	return Value{Type: TypeError, Str: fmt.Sprintf(format, a...)}
}

func Int(n int64) Value       { return Value{Type: TypeInteger, Num: n} }
func Bulk(s string) Value     { return Value{Type: TypeBulk, Str: s} }
func NullBulk() Value         { return Value{Type: TypeBulk, Null: true} }
func Array(vs ...Value) Value { return Value{Type: TypeArray, Elems: vs} }
func NullArray() Value        { return Value{Type: TypeArray, Null: true} }

// BulkStrings builds an Array of Bulk values from plain strings, the shape
// almost every command reply that returns a list uses.
func BulkStrings(ss []string) Value {
	elems := make([]Value, len(ss))
	for i, s := range ss {
		elems[i] = Bulk(s)
	}
	return Array(elems...)
}

// Marshal encodes v into its RESP2 wire representation.
func (v Value) Marshal() []byte {
	switch v.Type {
	case TypeSimpleString:
		return marshalLine('+', v.Str)
	case TypeError:
		return marshalLine('-', v.Str)
	case TypeInteger:
		return marshalLine(':', strconv.FormatInt(v.Num, 10))
	case TypeBulk:
		if v.Null {
			return []byte("$-1\r\n")
		}
		return marshalBulk(v.Str)
	case TypeArray:
		if v.Null {
			return []byte("*-1\r\n")
		}
		return marshalArray(v.Elems)
	default:
		return nil
	}
}

func marshalLine(prefix byte, s string) []byte {
	b := make([]byte, 0, len(s)+3)
	b = append(b, prefix)
	b = append(b, s...)
	b = append(b, '\r', '\n')
	return b
}

func marshalBulk(s string) []byte {
	b := make([]byte, 0, len(s)+16)
	b = append(b, '$')
	b = append(b, strconv.Itoa(len(s))...)
	b = append(b, '\r', '\n')
	b = append(b, s...)
	b = append(b, '\r', '\n')
	return b
}

func marshalArray(elems []Value) []byte {
	b := make([]byte, 0, 32)
	b = append(b, '*')
	b = append(b, strconv.Itoa(len(elems))...)
	b = append(b, '\r', '\n')
	for _, e := range elems {
		b = append(b, e.Marshal()...)
	}
	return b
}
