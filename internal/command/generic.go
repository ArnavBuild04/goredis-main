package command

import (
	"strconv"
	"time"

	"github.com/newpath/goredis/internal/resp"
	"github.com/newpath/goredis/internal/store"
)

func genericCommands() []Spec {
	return []Spec{
		{Name: "DEL", Handler: del, Arity: -1, MinArity: 1, Write: true},
		{Name: "EXISTS", Handler: exists, Arity: -1, MinArity: 1},
		{Name: "EXPIRE", Handler: expire, Arity: 2, Write: true},
		{Name: "PEXPIRE", Handler: pexpire, Arity: 2, Write: true},
		{Name: "TTL", Handler: ttl, Arity: 1},
		{Name: "PTTL", Handler: pttl, Arity: 1},
		{Name: "PERSIST", Handler: persist, Arity: 1, Write: true},
		{Name: "TYPE", Handler: typeCmd, Arity: 1},
		{Name: "KEYS", Handler: keys, Arity: 1},
		{Name: "RENAME", Handler: rename, Arity: 2, Write: true},
	}
}

func del(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	return resp.Int(int64(st.Del(argStrings(args)...)))
}

func exists(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	return resp.Int(int64(st.Exists(argStrings(args)...)))
}

func expire(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	seconds, err := strconv.ParseInt(args[1].Str, 10, 64)
	if err != nil {
		return resp.Errorf("ERR value is not an integer or out of range")
	}
	if st.Expire(args[0].Str, time.Duration(seconds)*time.Second) {
		return resp.Int(1)
	}
	return resp.Int(0)
}

func pexpire(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	millis, err := strconv.ParseInt(args[1].Str, 10, 64)
	if err != nil {
		return resp.Errorf("ERR value is not an integer or out of range")
	}
	if st.Expire(args[0].Str, time.Duration(millis)*time.Millisecond) {
		return resp.Int(1)
	}
	return resp.Int(0)
}

func ttl(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	return resp.Int(st.TTLSeconds(args[0].Str))
}

func pttl(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	return resp.Int(st.TTLMillis(args[0].Str))
}

func persist(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	if st.Persist(args[0].Str) {
		return resp.Int(1)
	}
	return resp.Int(0)
}

func typeCmd(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	kind, ok := st.Type(args[0].Str)
	if !ok {
		return resp.Str("none")
	}
	return resp.Str(string(kind))
}

func keys(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	return resp.BulkStrings(st.Keys(args[0].Str))
}

func rename(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	if err := st.Rename(args[0].Str, args[1].Str); err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.OK()
}

// argStrings extracts the .Str of every argument, the shape most variadic
// commands (DEL, EXISTS, MGET, SADD, ...) need their argument list in.
func argStrings(args []resp.Value) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = a.Str
	}
	return out
}
