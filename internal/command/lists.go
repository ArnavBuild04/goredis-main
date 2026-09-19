package command

import (
	"strconv"

	"github.com/newpath/goredis/internal/resp"
	"github.com/newpath/goredis/internal/store"
)

func listCommands() []Spec {
	return []Spec{
		{Name: "LPUSH", Handler: lpush, Arity: -1, MinArity: 2, Write: true},
		{Name: "RPUSH", Handler: rpush, Arity: -1, MinArity: 2, Write: true},
		{Name: "LPOP", Handler: lpop, Arity: 1, Write: true},
		{Name: "RPOP", Handler: rpop, Arity: 1, Write: true},
		{Name: "LLEN", Handler: llen, Arity: 1},
		{Name: "LRANGE", Handler: lrange, Arity: 3},
		{Name: "LINDEX", Handler: lindex, Arity: 2},
	}
}

func lpush(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	n, err := st.LPush(args[0].Str, argStrings(args[1:])...)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(int64(n))
}

func rpush(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	n, err := st.RPush(args[0].Str, argStrings(args[1:])...)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(int64(n))
}

func lpop(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	v, ok, err := st.LPop(args[0].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	if !ok {
		return resp.NullBulk()
	}
	return resp.Bulk(v)
}

func rpop(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	v, ok, err := st.RPop(args[0].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	if !ok {
		return resp.NullBulk()
	}
	return resp.Bulk(v)
}

func llen(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	n, err := st.LLen(args[0].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(int64(n))
}

func lrange(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	start, err1 := strconv.Atoi(args[1].Str)
	stop, err2 := strconv.Atoi(args[2].Str)
	if err1 != nil || err2 != nil {
		return resp.Errorf("ERR value is not an integer or out of range")
	}
	items, err := st.LRange(args[0].Str, start, stop)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.BulkStrings(items)
}

func lindex(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	idx, err := strconv.Atoi(args[1].Str)
	if err != nil {
		return resp.Errorf("ERR value is not an integer or out of range")
	}
	v, ok, err := st.LIndex(args[0].Str, idx)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	if !ok {
		return resp.NullBulk()
	}
	return resp.Bulk(v)
}
