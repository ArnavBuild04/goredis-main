package command

import (
	"github.com/newpath/goredis/internal/resp"
	"github.com/newpath/goredis/internal/store"
)

func connectionCommands() []Spec {
	return []Spec{
		{Name: "PING", Handler: ping, Arity: -1, MinArity: 0},
		{Name: "ECHO", Handler: echo, Arity: 1},
	}
}

func ping(_ *store.Store, _ *Context, args []resp.Value) resp.Value {
	if len(args) == 0 {
		return resp.Str("PONG")
	}
	return resp.Bulk(args[0].Str)
}

func echo(_ *store.Store, _ *Context, args []resp.Value) resp.Value {
	return resp.Bulk(args[0].Str)
}
