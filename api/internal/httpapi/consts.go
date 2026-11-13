package httpapi

import "time"

const (
	apiRoot        = "/api"
	apiPathPrefix  = apiRoot + "/"
	apiVersionPath = "/v1"
	apiPrefix      = apiRoot + apiVersionPath
	healthzPath    = "/healthz"
	readyzPath     = "/readyz"
	docsPath       = "/docs"
	docsURL        = apiRoot + docsPath + "/"
	docsPrefix     = apiRoot + docsPath
	specPath       = "/openapi.json"
	specURL        = apiRoot + specPath
	docsTitle      = "SimDesk API"
)

// Session cookie names. Browsers only accept the __Host- prefix on HTTPS.
const (
	cookieNameSecure   = "__Host-sd_session"
	cookieNameInsecure = "sd_session"
	cookiePath         = "/"
)

const (
	headerContentType     = "Content-Type"
	headerCacheControl    = "Cache-Control"
	headerRetryAfter      = "Retry-After"
	headerForwardedFor    = "X-Forwarded-For"
	headerRequestID       = "X-Request-Id"
	headerCSP             = "Content-Security-Policy"
	headerNoSniff         = "X-Content-Type-Options"
	headerReferrerPolicy  = "Referrer-Policy"
	headerFrameOptions    = "X-Frame-Options"
	headerPermissions     = "Permissions-Policy"
	headerHSTS            = "Strict-Transport-Security"
	contentTypeProblem    = "application/problem+json"
	contentTypeJSON       = "application/json"
	contentTypeHTML       = "text/html; charset=utf-8"
	cacheNoStore          = "no-store"
	cacheNoCache          = "no-cache"
	cacheImmutable        = "public, max-age=31536000, immutable"
	valueNoSniff          = "nosniff"
	valueReferrerPolicy   = "same-origin"
	valueFrameDeny        = "DENY"
	valuePermissions      = "camera=(), microphone=(), geolocation=()"
	valueHSTS             = "max-age=31536000"
	problemTypeBlank      = "about:blank"
	appCSP                = "default-src 'self'; img-src 'self' data: https:; style-src 'self'; script-src 'self'; connect-src 'self'; font-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"
	docsCSP               = "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'"
	forwardedForSeparator = ","
)

const (
	spaIndex        = "index.html"
	spaAssetsPrefix = "assets/"
	spaNotBuiltMsg  = "Web app not built. Run `make build`, or use the Vite dev server."
)

const (
	fieldPathSeparator    = "."
	fieldMessageSeparator = ": "
	invalidBodyDetail     = "Request body is not valid JSON for this operation."
)

const (
	maxUserAgentLen = 512
	readyzTimeout   = 2 * time.Second
)
