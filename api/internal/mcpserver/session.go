package mcpserver

import (
	"context"
	"errors"
	"math"
	"strconv"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
)

type sessionKey struct{}

func withSession(ctx context.Context, ts auth.TokenSession) context.Context {
	return context.WithValue(ctx, sessionKey{}, ts)
}

// sessionFrom is the token the request signed in with.
func sessionFrom(ctx context.Context) (auth.TokenSession, bool) {
	ts, ok := ctx.Value(sessionKey{}).(auth.TokenSession)
	return ts, ok && ts.Principal != nil
}

func asAppErr(err error, target **apperr.Error) bool { return errors.As(err, target) }

// retryAfterSeconds is a 429's Retry-After, in whole seconds rounded up.
func retryAfterSeconds(ae *apperr.Error) string {
	return strconv.Itoa(int(math.Ceil(ae.RetryAfter.Seconds())))
}
