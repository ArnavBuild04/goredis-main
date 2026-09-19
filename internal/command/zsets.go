package command

import (
	"strconv"
	"strings"

	"github.com/newpath/goredis/internal/resp"
	"github.com/newpath/goredis/internal/store"
)

func zsetCommands() []Spec {
	return []Spec{
		{Name: "ZADD", Handler: zadd, Arity: -1, MinArity: 3, Write: true},
		{Name: "ZSCORE", Handler: zscore, Arity: 2},
		{Name: "ZREM", Handler: zrem, Arity: -1, MinArity: 2, Write: true},
		{Name: "ZCARD", Handler: zcard, Arity: 1},
		{Name: "ZRANK", Handler: zrank, Arity: 2},
		{Name: "ZRANGE", Handler: zrange, Arity: -1, MinArity: 3},
	}
}

func zadd(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	if len(args)%2 != 1 {
		return resp.Errorf("ERR wrong number of arguments for 'zadd' command")
	}
	members := make(map[string]float64, (len(args)-1)/2)
	for i := 1; i < len(args); i += 2 {
		score, err := strconv.ParseFloat(args[i].Str, 64)
		if err != nil {
			return resp.Errorf("ERR value is not a valid float")
		}
		members[args[i+1].Str] = score
	}
	n, err := st.ZAdd(args[0].Str, members)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(int64(n))
}

func zscore(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	score, ok, err := st.ZScore(args[0].Str, args[1].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	if !ok {
		return resp.NullBulk()
	}
	return resp.Bulk(formatScore(score))
}

func zrem(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	n, err := st.ZRem(args[0].Str, argStrings(args[1:])...)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(int64(n))
}

func zcard(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	n, err := st.ZCard(args[0].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	return resp.Int(int64(n))
}

func zrank(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	rank, ok, err := st.ZRank(args[0].Str, args[1].Str)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}
	if !ok {
		return resp.NullBulk()
	}
	return resp.Int(int64(rank))
}

// zrange handles `ZRANGE key start stop [WITHSCORES]`.
func zrange(st *store.Store, _ *Context, args []resp.Value) resp.Value {
	start, err1 := strconv.Atoi(args[1].Str)
	stop, err2 := strconv.Atoi(args[2].Str)
	if err1 != nil || err2 != nil {
		return resp.Errorf("ERR value is not an integer or out of range")
	}

	withScores := false
	if len(args) == 4 {
		if strings.ToUpper(args[3].Str) != "WITHSCORES" {
			return resp.Errorf("ERR syntax error")
		}
		withScores = true
	} else if len(args) > 4 {
		return resp.Errorf("ERR syntax error")
	}

	members, err := st.ZRange(args[0].Str, start, stop)
	if err != nil {
		return resp.Errorf("%s", err.Error())
	}

	if !withScores {
		names := make([]string, len(members))
		for i, m := range members {
			names[i] = m.Member
		}
		return resp.BulkStrings(names)
	}

	elems := make([]resp.Value, 0, len(members)*2)
	for _, m := range members {
		elems = append(elems, resp.Bulk(m.Member), resp.Bulk(formatScore(m.Score)))
	}
	return resp.Array(elems...)
}

// formatScore renders a float the way Redis does: integral scores print
// without a decimal point.
func formatScore(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}
