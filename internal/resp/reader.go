package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

// Reader parses a stream of RESP2 frames off an underlying io.Reader.
type Reader struct {
	br *bufio.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{br: bufio.NewReader(r)}
}

// Read parses the next frame. It returns io.EOF (unwrapped) when the
// underlying stream is closed cleanly between frames.
func (r *Reader) Read() (Value, error) {
	typ, err := r.br.ReadByte()
	if err != nil {
		return Value{}, err
	}

	switch Type(typ) {
	case TypeArray:
		return r.readArray()
	case TypeBulk:
		return r.readBulk()
	case TypeSimpleString:
		line, err := r.readLine()
		if err != nil {
			return Value{}, err
		}
		return Str(line), nil
	case TypeError:
		line, err := r.readLine()
		if err != nil {
			return Value{}, err
		}
		return Value{Type: TypeError, Str: line}, nil
	case TypeInteger:
		n, err := r.readInteger()
		if err != nil {
			return Value{}, err
		}
		return Int(n), nil
	default:
		return Value{}, fmt.Errorf("resp: unknown type byte %q", typ)
	}
}

func (r *Reader) readLine() (string, error) {
	line, err := r.br.ReadString('\n')
	if err != nil {
		return "", err
	}
	n := len(line)
	if n < 2 || line[n-2] != '\r' {
		return "", fmt.Errorf("resp: malformed line %q", line)
	}
	return line[:n-2], nil
}

func (r *Reader) readInteger() (int64, error) {
	line, err := r.readLine()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(line, 10, 64)
}

func (r *Reader) readArray() (Value, error) {
	n, err := r.readInteger()
	if err != nil {
		return Value{}, err
	}
	if n < 0 {
		return NullArray(), nil
	}

	elems := make([]Value, n)
	for i := range elems {
		v, err := r.Read()
		if err != nil {
			return Value{}, err
		}
		elems[i] = v
	}
	return Array(elems...), nil
}

func (r *Reader) readBulk() (Value, error) {
	n, err := r.readInteger()
	if err != nil {
		return Value{}, err
	}
	if n < 0 {
		return NullBulk(), nil
	}

	buf := make([]byte, n)
	if _, err := io.ReadFull(r.br, buf); err != nil {
		return Value{}, err
	}
	if _, err := r.readLine(); err != nil {
		return Value{}, err
	}
	return Bulk(string(buf)), nil
}
