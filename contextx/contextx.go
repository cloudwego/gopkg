package contextx

import (
	"context"
	"sync"
)

type kvContextKeyType struct{}

var kvContextKey any = kvContextKeyType{}

type kvcontext struct {
	context.Context

	kvs sync.Map
}

// Value overwrites Context.Value for returning *kvcontext if key is kvContextKeyType
func (p *kvcontext) Value(key any) any {
	if key == kvContextKey {
		return p
	}
	if v, ok := p.kvs.Load(key); ok {
		return v
	}
	return p.Context.Value(key)
}

// UpdateValue creates or updates key, val pair of internal context without creating a context.
//
// It doesn't like context.WithValue which always create a new context.Context with one key/val.
// This optimizes a case like after calling context.WithValue many times,
// context.Value(key) can only be found in the deepest one.
// It will significantly make call stack shorter for retrieving a key.
//
// XXX: One of the side effects is the visiblity of key/val pair.
// Callees will be able to change values in callers context,
// this is an issue and a feature also ... use carefully and at your own rick.
func UpdateValue(parent context.Context, key, val any) context.Context {
	v := parent.Value(kvContextKey)
	ctx, ok := v.(*kvcontext)
	if !ok {
		ctx = &kvcontext{Context: parent}
	}
	ctx.kvs.Store(key, val)
	return ctx
}
