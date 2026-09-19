package command

import (
	"strconv"
	"strings"
	"time"

	"github.com/newpath/goredis/internal/resp"
	"github.com/newpath/goredis/internal/store"
)

func stringCommands() []Spec {
	return []Spec{
		{Name: "SET", Handler: set, Arity: -1, MinArity: 2, Write: true},
		{Name: "GET", Handler: get, Arity: 1},
		{Name: "GETSET", Handler: getSet, Arity: 2, Write: true},
		{Name: "SETNX", Handler: setNX, Arity: 2, Write: true},
		{Name: "SETEX", Handler: setEX, Arity: 3, Write: true},
		{Name: "MSET", Handler: mset, Arity: -1, MinArity: 2, Write: true},
		{Name: "MGET", Handler: mget, Arity: -1, MinArity: 1},
		{Name: "INCR", Handler: incr, Arity: 1, Write: true},
		{Name: "INCRBY", Handler: incrBy, Arity: 2, Write: true},
		{Name: "DECR", Handler: decr, Arity: 1, Write: true},
		{Name: "DECRBY", Handler: decrBy, Arity: 2, Write: true},
		{Name: "APPEND", Handler: appendCmd, Arity: 2, Write: true},
		{Name: "STRLEN", Handler: strlen, Arity: 1},
	}
}

// set parses `SET key value [EX seconds | PX milliseconds] [NX | XX]`.
func set(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	key, value := args[0].Str, args[1].Str
	var opts store.SetOptions

	for i := 2; i < len(args); i++ {
		switch strings.ToUpper(args[i].Str) {
		case "NX":
			opts.OnlyIfNew = true
		case "XX":
			opts.OnlyIfOld = true
		case "EX", "PX":
			unit := strings.ToUpper(args[i].Str)
			i++
			if i >= len(args) {
				return resp.Errorf("ERR syntax error")
			}
			n, err := strconv.ParseInt(args[i].Str, 10, 64)
			if err != nil {
				return resp.Errorf("ERR value is not an integer or out of range")
			}
			opts.HasTTL = true
			if unit == "EX" {
				opts.TTL = time.Duration(n) * time.Second
			} else {
				opts.TTL = time.Duration(n) * time.Millisecond
			}
		default:
			return resp.Errorf("ERR syntax error")
		}
	}

	ok, err := st.Set(key, value, opts)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	if !ok {
		return resp.NullBulk()
	}
	return resp.OK()
}

func get(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	v, found, err := st.Get(args[0].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	if !found {
		return resp.NullBulk()
	}
	return resp.Bulk(v)
}

func getSet(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	old, found, err := st.GetSet(args[0].Str, args[1].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	if !found {
		return resp.NullBulk()
	}
	return resp.Bulk(old)
}

func setNX(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	ok, err := st.Set(args[0].Str, args[1].Str, store.SetOptions{OnlyIfNew: true})
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	if ok {
		return resp.Int(1)
	}
	return resp.Int(0)
}

func setEX(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	seconds, err := strconv.ParseInt(args[1].Str, 10, 64)
	if err != nil {
		return resp.Errorf("ERR value is not an integer or out of range")
	}
	_, err = st.Set(args[0].Str, args[2].Str, store.SetOptions{HasTTL: true, TTL: time.Duration(seconds) * time.Second})
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.OK()
}

func mset(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	if len(args)%2 != 0 {
		return resp.Errorf("ERR wrong number of arguments for 'mset' command")
	}
	pairs := make(map[string]string, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		pairs[args[i].Str] = args[i+1].Str
	}
	st.MSet(pairs)
	return resp.OK()
}

func mget(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	results := st.MGet(argStrings(args))
	elems := make([]resp.Value, len(results))
	for i, v := range results {
		if v == nil {
			elems[i] = resp.NullBulk()
		} else {
			elems[i] = resp.Bulk(*v)
		}
	}
	return resp.Array(elems...)
}

func incr(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	return incrByAmount(st, args[0].Str, 1)
}

func incrBy(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	delta, err := strconv.ParseInt(args[1].Str, 10, 64)
	if err != nil {
		return resp.Errorf("ERR value is not an integer or out of range")
	}
	return incrByAmount(st, args[0].Str, delta)
}

func decr(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	return incrByAmount(st, args[0].Str, -1)
}

func decrBy(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	delta, err := strconv.ParseInt(args[1].Str, 10, 64)
	if err != nil {
		return resp.Errorf("ERR value is not an integer or out of range")
	}
	return incrByAmount(st, args[0].Str, -delta)
}

func incrByAmount(st *store.Store, key string, delta int64) resp.Value {
	n, err := st.IncrBy(key, delta)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(n)
}

func appendCmd(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	n, err := st.Append(args[0].Str, args[1].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(int64(n))
}

func strlen(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	n, err := st.StrLen(args[0].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(int64(n))
}
