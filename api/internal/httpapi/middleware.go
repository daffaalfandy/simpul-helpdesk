package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/reqinfo"
)

// ClientIP returns the client address. X-Forwarded-For is only honoured when
// the direct peer is a trusted proxy; the right-most untrusted hop wins.
func ClientIP(remoteAddr string, xff []string, trusted []netip.Prefix) netip.Addr {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	peer = peer.Unmap()
	if !isTrusted(peer, trusted) {
		return peer
	}
	var hops []netip.Addr
	for _, h := range xff {
		for part := range strings.SplitSeq(h, forwardedForSeparator) {
			if a, err := netip.ParseAddr(strings.TrimSpace(part)); err == nil {
				hops = append(hops, a.Unmap())
			}
		}
	}
	for i := len(hops) - 1; i >= 0; i-- {
		if !isTrusted(hops[i], trusted) {
			return hops[i]
		}
	}
	if len(hops) > 0 {
		return hops[0]
	}
	return peer
}

func isTrusted(a netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// withRequestID assigns a random request ID and echoes it in X-Request-Id.
// (chi's RequestID middleware prefixes the hostname, which leaks to clients.)
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		w.Header().Set(headerRequestID, id)
		ctx := context.WithValue(r.Context(), middleware.RequestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// withRequestInfo stores client IP, user agent and request ID in the context.
func withRequestInfo(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ua := r.UserAgent()
			if len(ua) > maxUserAgentLen {
				ua = ua[:maxUserAgentLen]
			}
			info := reqinfo.Info{
				IP:        ClientIP(r.RemoteAddr, r.Header.Values(headerForwardedFor), trusted),
				UserAgent: ua,
				RequestID: middleware.GetReqID(r.Context()),
			}
			next.ServeHTTP(w, r.WithContext(reqinfo.With(r.Context(), info)))
		})
	}
}

// requestLogger logs one line per request (health checks excluded).
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			if r.URL.Path == healthzPath || r.URL.Path == readyzPath {
				return
			}
			log.InfoContext(r.Context(), "request",
				"method", r.Method, "path", r.URL.Path, "status", ww.Status(),
				"bytes", ww.BytesWritten(), "duration_ms", time.Since(start).Milliseconds(),
				"ip", reqinfo.From(r.Context()).IP.String(), "request_id", middleware.GetReqID(r.Context()))
		})
	}
}

// recoverer turns panics into a logged 500 problem.
func recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					if v == http.ErrAbortHandler {
						panic(v)
					}
					log.ErrorContext(r.Context(), "panic", "value", v, "path", r.URL.Path,
						"request_id", middleware.GetReqID(r.Context()))
					writeAppError(w, r, apperr.Internal())
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// securityHeaders sets defensive headers on every response.
func securityHeaders(hsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set(headerNoSniff, valueNoSniff)
			h.Set(headerReferrerPolicy, valueReferrerPolicy)
			h.Set(headerFrameOptions, valueFrameDeny)
			h.Set(headerPermissions, valuePermissions)
			if strings.HasPrefix(r.URL.Path, docsPrefix) {
				h.Set(headerCSP, docsCSP)
			} else {
				h.Set(headerCSP, appCSP)
			}
			if hsts {
				h.Set(headerHSTS, valueHSTS)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// session loads the signed-in user from the session cookie. It never rejects;
// operations that need a session are guarded by requireAuth.
func (s *server) session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(s.cookieName)
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		p, err := s.auth.Authenticate(r.Context(), c.Value)
		switch {
		case err == nil:
			r = r.WithContext(auth.WithPrincipal(r.Context(), p))
		case isUnauthenticated(err):
			http.SetCookie(w, s.clearCookie())
		default:
			s.log.ErrorContext(r.Context(), "authenticate session", "err", err)
		}
		next.ServeHTTP(w, r)
	})
}

func isUnauthenticated(err error) bool {
	var ae *apperr.Error
	return errors.As(err, &ae) && ae.Status == http.StatusUnauthorized
}
