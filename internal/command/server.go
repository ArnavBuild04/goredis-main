package command

import (
	"fmt"
	"runtime"

	"github.com/newpath/goredis/internal/resp"
	"github.com/newpath/goredis/internal/store"
)

func serverCommands() []Spec {
	return []Spec{
		{Name: "FLUSHALL", Handler: flushall, Arity: 0, Write: true},
		{Name: "DBSIZE", Handler: dbsize, Arity: 0},
		{Name: "INFO", Handler: info, Arity: -1, MinArity: 0},
	}
}

func flushall(st *store.Store, _ *Context, _ []resp.Value) resp.Value {
	st.FlushAll()
	return resp.OK()
}

func dbsize(st *store.Store, _ *Context, _ []resp.Value) resp.Value {
	return resp.Int(int64(st.DBSize()))
}

func info(st *store.Store, _ *Context, _ []resp.Value) resp.Value {
	body := fmt.Sprintf(
		"# Server\r\ngoredis_version:1.0.0\r\ngo_version:%s\r\nos:%s/%s\r\n\r\n# Keyspace\r\ndb0:keys=%d\r\n",
		runtime.Version(), runtime.GOOS, runtime.GOARCH, st.DBSize(),
	)
	return resp.Bulk(body)
}
