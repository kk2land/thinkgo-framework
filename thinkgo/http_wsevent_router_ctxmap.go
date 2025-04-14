package thinkgo

import (
	"github.com/gin-gonic/gin"
	"strings"
	"sync/atomic"
	"time"
)

type wsEventRouterCtxMap interface {
	loadCtx(key string, logger FieldLogger, c *gin.Context) *wsEventContext
	deleteCtx(ctx *wsEventContext, now time.Time) bool
	Range(f func(key string, ctx *wsEventContext), paramsString string)
	RangeByKeys(f func(key string, ctx *wsEventContext), keys ...string)
}

type wsEventRouterCtxMapDefault struct {
	router *wsEventRouter
	cMap   *CMap[*wsEventContext] // { key => ctx }
}

func (m *wsEventRouterCtxMapDefault) loadCtx(key string, logger FieldLogger, c *gin.Context) *wsEventContext {
	if atomic.LoadInt32(&m.router.closed) == 1 {
		panic(wsEventRouterCloseErr)
	}
	l := wsEventLogger(logger, key)
	shard := m.cMap.GetShard(key)
	ctx, ok := shard.LoadOrCreate(key, func(k string) *wsEventContext {
		return newWsEventContext(key, "", m.router, shard, l)
	})
	if ok {
		ctx.atime = time.Now()
		ctx.logger = l
	} else {
		go ctx.goStart()
		m.router.ctxWait.Add(1)
	}
	return ctx
}

func (m *wsEventRouterCtxMapDefault) deleteCtx(ctx *wsEventContext, now time.Time) bool {
	shard := ctx.routerParams.(*CMapShard[*CMap[*wsEventContext]])
	shard.Lock()
	defer shard.Unlock()
	if atomic.LoadInt32(&m.router.closed) == 0 && now.Sub(ctx.atime) <= m.router.ctxKeepalive {
		return false
	}
	shard.Delete(ctx.key)
	return true
}

func (m *wsEventRouterCtxMapDefault) Range(f func(key string, ctx *wsEventContext), paramsString string) {
	m.cMap.Range(func(k string, v *wsEventContext) bool {
		f(k, v)
		return true
	})
}

func (m *wsEventRouterCtxMapDefault) RangeByKeys(f func(key string, ctx *wsEventContext), keys ...string) {
	for _, k := range keys {
		if v, ok := m.cMap.Load(k); ok {
			f(k, v)
		}
	}
}

func wsEventRouterParamsString(p gin.Params) string {
	buf := BytesBuffer1024.Get()
	defer BytesBuffer1024.Put(buf)
	for _, entry := range p {
		buf.WriteByte('/')
		buf.WriteString(entry.Value)
	}
	return buf.String()
}

func wsEventRouterCtxMapParamsEntry(k string) *CMap[*wsEventContext] {
	return CMapNew[*wsEventContext]()
}

type wsEventRouterCtxMapParams struct {
	router *wsEventRouter
	cMap   *CMap[*CMap[*wsEventContext]] // { paramsString => { key => ctx } }
}

func (m *wsEventRouterCtxMapParams) loadCtx(key string, logger FieldLogger, c *gin.Context) *wsEventContext {
	var ctx *wsEventContext
	if atomic.LoadInt32(&m.router.closed) == 1 {
		panic(wsEventRouterCloseErr)
	}
	paramsString := wsEventRouterParamsString(c.Params)
	key = WsEventKey(key, paramsString)
	l := wsEventLogger(logger, key)
	shard := m.cMap.GetShard(paramsString)
	shard.LoadOrCreateCb(paramsString, wsEventRouterCtxMapParamsEntry, func(ctxMap *CMap[*wsEventContext], loaded bool) {
		ctx, ok := ctxMap.LoadOrCreate(key, func(k string) *wsEventContext {
			return newWsEventContext(key, paramsString, m.router, shard, l)
		})
		if ok {
			ctx.atime = time.Now()
			ctx.logger = l
		} else {
			println("a1")
			go ctx.goStart()
			m.router.ctxWait.Add(1)
		}
	})
	return ctx
}

func (m *wsEventRouterCtxMapParams) deleteCtx(ctx *wsEventContext, now time.Time) bool {
	shard := ctx.routerParams.(*CMapShard[*CMap[*wsEventContext]])
	shard.Lock()
	defer shard.Unlock()
	if atomic.LoadInt32(&m.router.closed) == 0 && now.Sub(ctx.atime) <= m.router.ctxKeepalive {
		return false
	}
	if ctxMap, ok := shard.Items()[ctx.paramsString]; ok {
		ctxMapShard := ctxMap.GetShard(ctx.key)
		ctxMapShard.DeleteCb(ctx.key, func(items map[string]*wsEventContext) {
			if len(items) == 0 {
				delete(shard.Items(), ctx.paramsString)
			}
		})
	}
	return true
}

func (m *wsEventRouterCtxMapParams) Range(f func(key string, ctx *wsEventContext), paramsString string) {
	var ctxMap *CMap[*wsEventContext]
	var ok bool
	cb := func(k string, v *wsEventContext) bool {
		f(k, v)
		return true
	}
	if len(paramsString) > 0 {
		if ctxMap, ok = m.cMap.Load(paramsString); ok {
			ctxMap.Range(cb)
		}
	} else {
		m.cMap.Range(func(k string, v *CMap[*wsEventContext]) bool {
			v.Range(cb)
			return true
		})
	}
}

func (m *wsEventRouterCtxMapParams) RangeByKeys(f func(key string, ctx *wsEventContext), keys ...string) {
	keysMap := make(map[string][]string)
	for _, k := range keys {
		paramsString := strings.SplitN(k, "^", 2)[0]
		keysMap[paramsString] = append(keysMap[paramsString], k)
	}
	var v *wsEventContext
	for paramsString, keys1 := range keysMap {
		if ctxMap, ok := m.cMap.Load(paramsString); ok {
			for _, k := range keys1 {
				if v, ok = ctxMap.Load(k); ok {
					f(k, v)
				}
			}
		}
	}
}
