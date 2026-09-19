package command

import (
	"github.com/newpath/goredis/internal/resp"
	"github.com/newpath/goredis/internal/store"
)

func pubsubCommands() []Spec {
	return []Spec{
		{Name: "SUBSCRIBE", Handler: subscribe, Arity: -1, MinArity: 1},
		{Name: "UNSUBSCRIBE", Handler: unsubscribe, Arity: -1, MinArity: 0},
		{Name: "PUBLISH", Handler: publish, Arity: 2, Write: true},
	}
}

func subscribe(_ *store.Store, ctx *Context, args []resp.Value) resp.Value {
	sub := ctx.Subscriber()
	frames := make([]resp.Value, len(args))
	for i, a := range args {
		count := ctx.Hub.Subscribe(sub, a.Str)
		frames[i] = resp.Array(resp.Bulk("subscribe"), resp.Bulk(a.Str), resp.Int(int64(count)))
	}
	return resp.MultiFrame(frames...)
}

func unsubscribe(_ *store.Store, ctx *Context, args []resp.Value) resp.Value {
	sub := ctx.Subscriber()
	channels := argStrings(args)
	if len(channels) == 0 {
		channels = sub.Channels()
	}
	if len(channels) == 0 {
		return resp.MultiFrame(resp.Array(resp.Bulk("unsubscribe"), resp.NullBulk(), resp.Int(0)))
	}

	frames := make([]resp.Value, len(channels))
	for i, c := range channels {
		count := ctx.Hub.Unsubscribe(sub, c)
		frames[i] = resp.Array(resp.Bulk("unsubscribe"), resp.Bulk(c), resp.Int(int64(count)))
	}
	return resp.MultiFrame(frames...)
}

func publish(_ *store.Store, ctx *Context, args []resp.Value) resp.Value {
	n := ctx.Hub.Publish(args[0].Str, args[1].Str)
	return resp.Int(int64(n))
}
