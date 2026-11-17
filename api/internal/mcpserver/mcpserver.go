// Package mcpserver is SimDesk's MCP server (owner, 17 Nov): staff connect an AI client with a
// personal access token, read tickets and, with a read-and-write token, reply, add internal
// notes and change status, assignee and priority.
//
// Every tool calls the same services as the app with the token's user as the principal, so
// access rules, the audit log, notifications and emails behave exactly as they do there.
package mcpserver

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/clients"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// Server identity sent in the MCP handshake.
const (
	serverName    = "simdesk"
	serverTitle   = "SimDesk"
	serverVersion = "1.0.0"
)

// HTTP auth: the bearer scheme in Authorization, and the challenge sent with a 401.
const (
	headerAuthorization   = "Authorization"
	headerAuthenticate    = "WWW-Authenticate"
	bearerScheme          = "Bearer"
	bearerChallenge       = `Bearer realm="simdesk"`
	headerRetryAfter      = "Retry-After"
	unauthenticatedDetail = "Sign in with a SimDesk personal access token: Authorization: Bearer simdesk_pat_…"
)

// instructions tell the client's model what SimDesk is and how its tickets read.
const instructions = `SimDesk is a helpdesk where an IT vendor's staff work tickets from client organisations.
Tickets have keys such as HD-24871. Descriptions and messages are Markdown.
A reply goes to the client and is emailed to them; an internal note is for the vendor's staff only and never reaches the client.
Use search_tickets to find tickets, get_ticket for the description and conversation, and read_attachment for screenshots and logs.`

// Deps are the services the tools call.
type Deps struct {
	Auth    *auth.Service
	Tickets *tickets.Service
	Clients *clients.Service
	// AppURL is where the app lives, for links back to tickets.
	AppURL *url.URL
	// MaxBodyBytes caps a request body.
	MaxBodyBytes int64
	Logger       *slog.Logger
}

// New returns the MCP endpoint. Each request signs in with its bearer token and gets a server
// of its own (stateless: nothing is kept between requests), with the write tools only for a
// read-and-write token.
func New(d Deps) http.Handler {
	cache := mcp.NewSchemaCache()
	h := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		ts, ok := sessionFrom(r.Context())
		if !ok {
			return nil
		}
		srv := mcp.NewServer(&mcp.Implementation{Name: serverName, Title: serverTitle, Version: serverVersion},
			&mcp.ServerOptions{Instructions: instructions, Logger: d.Logger, SchemaCache: cache})
		t := &toolset{deps: d, session: ts}
		t.register(srv)
		return srv
	}, &mcp.StreamableHTTPOptions{
		Stateless:           true,
		JSONResponse:        true,
		Logger:              d.Logger,
		MaxRequestBodyBytes: d.MaxBodyBytes,
		// Every request needs a personal token, which a DNS-rebinding page can't know; the
		// check would refuse a reverse proxy on the same machine that keeps the public Host.
		DisableLocalhostProtection: true,
	})
	return d.bearer(h)
}

// bearer signs the request in with its personal access token, or answers 401 (429 when the
// address has sent too many unknown tokens). Session cookies are never read here.
func (d Deps) bearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r.Header.Get(headerAuthorization))
		if !ok {
			unauthorized(w)
			return
		}
		ts, err := d.Auth.AuthenticateToken(r.Context(), raw)
		if err != nil {
			var ae *apperr.Error
			switch {
			case apperr.IsCode(err, apperr.CodeRateLimited) && asAppErr(err, &ae):
				w.Header().Set(headerRetryAfter, retryAfterSeconds(ae))
				http.Error(w, ae.Title, http.StatusTooManyRequests)
			case apperr.IsCode(err, apperr.CodeUnauthenticated):
				unauthorized(w)
			default:
				d.Logger.ErrorContext(r.Context(), "mcp: authenticate token", "err", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}
		next.ServeHTTP(w, r.WithContext(withSession(r.Context(), ts)))
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set(headerAuthenticate, bearerChallenge)
	http.Error(w, unauthenticatedDetail, http.StatusUnauthorized)
}

// bearerToken reads the token from an Authorization header value. The scheme is
// case-insensitive (RFC 7235); the token is not.
func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok || !strings.EqualFold(scheme, bearerScheme) {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}
