// Package httpapi wires the HTTP router: middleware, the generated OpenAPI
// server, Swagger UI and the embedded web app.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
	"github.com/swaggest/swgui/v5emb"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/catalog"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/clients"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/config"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

type Deps struct {
	Config  config.Config
	Store   *store.Store
	Auth    *auth.Service
	Clients *clients.Service
	SLA     *sla.Service
	Catalog *catalog.Service
	Tickets *tickets.Service
	Logger  *slog.Logger
	WebUI   fs.FS // built SPA; may be empty in development
}

type server struct {
	cfg        config.Config
	store      *store.Store
	auth       *auth.Service
	clients    *clients.Service
	sla        *sla.Service
	catalog    *catalog.Service
	tickets    *tickets.Service
	log        *slog.Logger
	cookieName string
}

var _ gen.StrictServerInterface = (*server)(nil)

func New(d Deps) (http.Handler, error) {
	spec, err := gen.GetSpec()
	if err != nil {
		return nil, err
	}
	s := &server{
		cfg: d.Config, store: d.Store, auth: d.Auth, clients: d.Clients, sla: d.SLA, catalog: d.Catalog,
		tickets: d.Tickets, log: d.Logger, cookieName: cookieName(d.Config.CookieSecure),
	}
	streamed := streamedRoutes(spec)

	r := chi.NewRouter()
	r.Use(withRequestID)
	r.Use(withRequestInfo(d.Config.TrustedProxies))
	r.Use(requestLogger(d.Logger))
	r.Use(recoverer(d.Logger))
	r.Use(securityHeaders(d.Config.CookieSecure))
	cop := http.NewCrossOriginProtection()
	cop.SetDenyHandler(problemHandler(apperr.CrossOrigin))
	r.Use(cop.Handler)
	r.MethodNotAllowed(problemHandler(apperr.MethodNotAllowed))

	r.Get(healthzPath, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	r.Get(readyzPath, func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readyzTimeout)
		defer cancel()
		if err := d.Store.Ping(ctx); err != nil {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	r.Route(apiRoot, func(api chi.Router) {
		api.NotFound(problemHandler(apperr.NotFound))
		api.MethodNotAllowed(problemHandler(apperr.MethodNotAllowed))
		if d.Config.DocsEnabled {
			mountDocs(api, spec)
		}
		api.Route(apiVersionPath, func(v1 chi.Router) {
			v1.Use(s.session)
			v1.Use(bodyLimits(streamed))
			v1.Use(s.validator(streamed))
			middlewares := []gen.StrictMiddlewareFunc{downloadDeadline(binaryResponseOps(spec)), requireAuth(publicOperations(spec))}
			strict := gen.NewStrictHandlerWithOptions(s, middlewares,
				gen.StrictHTTPServerOptions{
					RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
						if tooLargeError(err) {
							writeAppError(w, r, errRequestTooLarge())
							return
						}
						writeAppError(w, r, apperr.InvalidRequest(err.Error()))
					},
					ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
						writeProblem(w, r, s.log, err)
					},
				})
			gen.HandlerWithOptions(strict, gen.ChiServerOptions{
				BaseRouter: v1,
				ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
					writeAppError(w, r, apperr.InvalidRequest(err.Error()))
				},
			})
		})
	})

	r.NotFound(spaHandler(d.WebUI, d.Logger).ServeHTTP)
	return r, nil
}

// validator checks requests against the OpenAPI spec before they reach handlers. Streamed
// (multipart) routes skip it; see streamedRoutes.
func (s *server) validator(streamed map[string]bool) func(http.Handler) http.Handler {
	spec, err := gen.GetSpec() // fresh copy: the middleware mutates Servers
	if err != nil {
		panic(err)
	}
	return nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
		DoNotValidateServers:  true,
		SilenceServersWarning: true,
		Prefix:                apiPrefix,
		Skipper:               func(r *http.Request) bool { return streamed[routeKey(r.Method, r.URL.Path)] },
		Options: openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc, // the session middleware owns auth
		},
		ErrorHandlerWithOpts: func(_ context.Context, err error, w http.ResponseWriter, r *http.Request, opts nethttpmiddleware.ErrorHandlerOpts) {
			switch {
			case opts.MatchedRoute == nil && opts.StatusCode == http.StatusMethodNotAllowed:
				writeAppError(w, r, apperr.MethodNotAllowed())
			case opts.MatchedRoute == nil:
				writeAppError(w, r, apperr.NotFound())
			default:
				writeAppError(w, r, validationError(err))
			}
		},
	})
}

// validationError turns a spec validation failure into a 400 with per-field
// errors, without echoing the schema back to the client.
func validationError(err error) *apperr.Error {
	if tooLargeError(err) {
		return errRequestTooLarge()
	}
	var se *openapi3.SchemaError
	if errors.As(err, &se) {
		ae := apperr.InvalidRequest(se.Reason)
		if field := strings.Join(se.JSONPointer(), fieldPathSeparator); field != "" {
			ae.Detail = field + fieldMessageSeparator + se.Reason
			ae.Fields = []apperr.FieldError{{Field: field, Message: se.Reason}}
		}
		return ae
	}
	var re *openapi3filter.RequestError
	if errors.As(err, &re) {
		if re.Parameter != nil {
			return apperr.InvalidRequest(re.Parameter.Name + fieldMessageSeparator + re.Reason)
		}
		if re.RequestBody != nil {
			return apperr.InvalidRequest(invalidBodyDetail)
		}
		return apperr.InvalidRequest(re.Reason)
	}
	return apperr.InvalidRequest("")
}

// publicOperations lists the Go operation names marked `security: []` in the spec.
func publicOperations(spec *openapi3.T) map[string]bool {
	public := map[string]bool{}
	for _, item := range spec.Paths.Map() {
		for _, op := range item.Operations() {
			if op.Security != nil && len(*op.Security) == 0 {
				public[goName(op.OperationID)] = true
			}
		}
	}
	return public
}

// goName matches oapi-codegen's handler naming (getMe -> GetMe).
func goName(operationID string) string {
	if operationID == "" {
		return ""
	}
	r := []rune(operationID)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// requireAuth rejects anonymous requests to every operation that isn't public,
// so new endpoints are protected by default.
func requireAuth(public map[string]bool) gen.StrictMiddlewareFunc {
	return func(f gen.StrictHandlerFunc, operationID string) gen.StrictHandlerFunc {
		if public[operationID] {
			return f
		}
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, req any) (any, error) {
			if _, ok := auth.PrincipalFrom(ctx); !ok {
				return nil, apperr.Unauthenticated()
			}
			return f(ctx, w, r, req)
		}
	}
}

func mountDocs(api chi.Router, spec *openapi3.T) {
	specJSON, err := json.Marshal(spec)
	if err != nil {
		panic(err)
	}
	api.Get(specPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(headerContentType, contentTypeJSON)
		_, _ = w.Write(specJSON)
	})
	api.Get(docsPath, http.RedirectHandler(docsURL, http.StatusMovedPermanently).ServeHTTP)
	api.Handle(docsPath+"/*", v5emb.New(docsTitle, specURL, docsURL))
}

func cookieName(secure bool) string {
	if secure {
		return cookieNameSecure
	}
	return cookieNameInsecure
}

func (s *server) sessionCookie(token string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name: s.cookieName, Value: token, Path: cookiePath, Expires: expires,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	}
}

func (s *server) clearCookie() *http.Cookie {
	return &http.Cookie{
		Name: s.cookieName, Value: "", Path: cookiePath, MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	}
}

func isAPIPath(p string) bool { return p == apiRoot || strings.HasPrefix(p, apiPathPrefix) }
