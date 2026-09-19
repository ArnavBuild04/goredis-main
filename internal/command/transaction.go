package command

import (
	"github.com/newpath/goredis/internal/resp"
	"github.com/newpath/goredis/internal/store"
)

func transactionCommands() []Spec {
	return []Spec{
		{Name: "MULTI", Handler: multi, Arity: 0},
		{Name: "EXEC", Handler: exec, Arity: 0},
		{Name: "DISCARD", Handler: discard, Arity: 0},
	}
}

func multi(_ *store.Store, ctx *Context, _ []resp.Value) resp.Value {
	if ctx.inTx {
		return resp.Errorf("ERR MULTI calls can not be nested")
	}
	ctx.inTx = true
	ctx.queue = nil
	ctx.txError = false
	return resp.OK()
}

// exec runs every queued command in order and returns their replies as one
// array. There is no rollback on a mid-transaction error, matching Redis:
// individual command failures inside EXEC are reported per-element, not
// treated as reasons to undo earlier commands in the same batch.
func exec(st *store.Store, ctx *Context, _ []resp.Value) resp.Value {
	if !ctx.inTx {
		return resp.Errorf("ERR EXEC without MULTI")
	}
	queue, dirty := ctx.queue, ctx.txError
	ctx.inTx, ctx.queue, ctx.txError = false, nil, false

	if dirty {
		return resp.Errorf("EXECABORT Transaction discarded because of previous errors.")
	}

	results := make([]resp.Value, len(queue))
	for i, qc := range queue {
		results[i] = ctx.Registry.Dispatch(st, ctx, qc.Name, qc.Args)
	}
	return resp.Array(results...)
}

func discard(_ *store.Store, ctx *Context, _ []resp.Value) resp.Value {
	if !ctx.inTx {
		return resp.Errorf("ERR DISCARD without MULTI")
	}
	ctx.inTx, ctx.queue, ctx.txError = false, nil, false
	return resp.OK()
}
