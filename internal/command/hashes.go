package command

import (
	"strconv"

	"github.com/newpath/goredis/internal/resp"
	"github.com/newpath/goredis/internal/store"
)

func hashCommands() []Spec {
	return []Spec{
		{Name: "HSET", Handler: hset, Arity: -1, MinArity: 3, Write: true},
		{Name: "HGET", Handler: hget, Arity: 2},
		{Name: "HGETALL", Handler: hgetall, Arity: 1},
		{Name: "HDEL", Handler: hdel, Arity: -1, MinArity: 2, Write: true},
		{Name: "HEXISTS", Handler: hexists, Arity: 2},
		{Name: "HLEN", Handler: hlen, Arity: 1},
		{Name: "HKEYS", Handler: hkeys, Arity: 1},
		{Name: "HVALS", Handler: hvals, Arity: 1},
		{Name: "HMGET", Handler: hmget, Arity: -1, MinArity: 2},
		{Name: "HINCRBY", Handler: hincrby, Arity: 3, Write: true},
	}
}

func hset(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	if len(args)%2 != 1 {
		return resp.Errorf("ERR wrong number of arguments for 'hset' command")
	}
	fields := make(map[string]string, (len(args)-1)/2)
	for i := 1; i < len(args); i += 2 {
		fields[args[i].Str] = args[i+1].Str
	}
	n, err := st.HSet(args[0].Str, fields)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(int64(n))
}

func hget(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	v, ok, err := st.HGet(args[0].Str, args[1].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	if !ok {
		return resp.NullBulk()
	}
	return resp.Bulk(v)
}

func hgetall(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	h, err := st.HGetAll(args[0].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	elems := make([]resp.Value, 0, len(h)*2)
	for k, v := range h {
		elems = append(elems, resp.Bulk(k), resp.Bulk(v))
	}
	return resp.Array(elems...)
}

func hdel(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	n, err := st.HDel(args[0].Str, argStrings(args[1:])...)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(int64(n))
}

func hexists(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	ok, err := st.HExists(args[0].Str, args[1].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	if ok {
		return resp.Int(1)
	}
	return resp.Int(0)
}

func hlen(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	n, err := st.HLen(args[0].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(int64(n))
}

func hkeys(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	ks, err := st.HKeys(args[0].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.BulkStrings(ks)
}

func hvals(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	vs, err := st.HVals(args[0].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.BulkStrings(vs)
}

func hmget(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	results, err := st.HMGet(args[0].Str, argStrings(args[1:]))
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
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

func hincrby(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	delta, err := strconv.ParseInt(args[2].Str, 10, 64)
	if err != nil {
		return resp.Errorf("ERR value is not an integer or out of range")
	}
	n, err := st.HIncrBy(args[0].Str, args[1].Str, delta)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(n)
}
