// Package reqinfo carries request metadata (client IP, user agent, request ID) in the context.
package reqinfo

import (
	"context"
	"net/netip"
)

// Info describes the HTTP request that triggered the current work.
type Info struct {
	IP        netip.Addr
	UserAgent string
	RequestID string
}

type key struct{}

func With(ctx context.Context, info Info) context.Context {
	return context.WithValue(ctx, key{}, info)
}

// From returns the request info, or the zero value outside a request.
func From(ctx context.Context) Info {
	info, _ := ctx.Value(key{}).(Info)
	return info
}
