package command

import (
	"github.com/newpath/goredis/internal/resp"
	"github.com/newpath/goredis/internal/store"
)

func setCommands() []Spec {
	return []Spec{
		{Name: "SADD", Handler: sadd, Arity: -1, MinArity: 2, Write: true},
		{Name: "SREM", Handler: srem, Arity: -1, MinArity: 2, Write: true},
		{Name: "SMEMBERS", Handler: smembers, Arity: 1},
		{Name: "SISMEMBER", Handler: sismember, Arity: 2},
		{Name: "SCARD", Handler: scard, Arity: 1},
	}
}

func sadd(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	n, err := st.SAdd(args[0].Str, argStrings(args[1:])...)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(int64(n))
}

func srem(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	n, err := st.SRem(args[0].Str, argStrings(args[1:])...)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(int64(n))
}

func smembers(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	members, err := st.SMembers(args[0].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.BulkStrings(members)
}

func sismember(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	ok, err := st.SIsMember(args[0].Str, args[1].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	if ok {
		return resp.Int(1)
	}
	return resp.Int(0)
}

func scard(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	n, err := st.SCard(args[0].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(int64(n))
}
