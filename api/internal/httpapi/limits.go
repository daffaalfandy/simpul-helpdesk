package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/connector"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// Uploads carry one file plus multipart framing. Transfers get their own deadline because the
// server-wide read and write timeouts would cut a 10 MB transfer on a slow link.
const (
	maxJSONBodyBytes       = 1 << 20  // 1 MiB
	multipartOverheadBytes = 64 << 10 // part headers and boundaries
	maxUploadBodyBytes     = tickets.MaxAttachmentBytes + multipartOverheadBytes
	maxWebhookBodyBytes    = connector.MaxDeliveryBytes
	transferTimeout        = 10 * time.Minute

	contentTypeMultipart    = "multipart/form-data"
	schemaFormatBinary      = "binary"
	pathParamOpen           = "{"
	pathSeparator           = "/"
	successStatusClassFirst = "2"
	// extRawBody marks an operation whose handler reads the body as sent (a webhook signed over
	// its exact bytes).
	extRawBody = "x-raw-body"
)

// rawRoute is an operation whose handler reads the body itself: multipart uploads and webhooks.
// They skip the validator, which reads whole bodies into memory, and get their own size limit.
type rawRoute struct {
	method   string
	segments []string // the path template; "{…}" matches any one segment
	upload   bool     // a signed-in upload: the multipart limit and the longer transfer deadline
}

type rawRoutes []rawRoute

// rawBodyRoutes lists the multipart and x-raw-body operations, read from the spec.
func rawBodyRoutes(spec *openapi3.T) rawRoutes {
	var out rawRoutes
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			multipart := op.RequestBody != nil && op.RequestBody.Value != nil &&
				op.RequestBody.Value.Content.Get(contentTypeMultipart) != nil
			raw, _ := op.Extensions[extRawBody].(bool)
			if multipart || raw {
				out = append(out, rawRoute{method: method, segments: strings.Split(apiPrefix+path, pathSeparator), upload: multipart})
			}
		}
	}
	return out
}

// match finds the raw-body route a request is for.
func (rs rawRoutes) match(r *http.Request) (rawRoute, bool) {
	segments := strings.Split(r.URL.Path, pathSeparator)
	for _, route := range rs {
		if route.method == r.Method && route.matches(segments) {
			return route, true
		}
	}
	return rawRoute{}, false
}

func (r rawRoute) matches(segments []string) bool {
	if len(segments) != len(r.segments) {
		return false
	}
	for i, want := range r.segments {
		if !strings.HasPrefix(want, pathParamOpen) && want != segments[i] {
			return false
		}
	}
	return true
}

// binaryResponseOps lists the operations (Go names) that answer with a file: a success
// response whose body is a binary string, whatever its media type (attachments, exports).
func binaryResponseOps(spec *openapi3.T) map[string]bool {
	out := map[string]bool{}
	for _, item := range spec.Paths.Map() {
		for _, op := range item.Operations() {
			for status, resp := range op.Responses.Map() {
				if strings.HasPrefix(status, successStatusClassFirst) && resp.Value != nil && sendsFile(resp.Value.Content) {
					out[goName(op.OperationID)] = true
				}
			}
		}
	}
	return out
}

func sendsFile(content openapi3.Content) bool {
	for _, media := range content {
		if media.Schema != nil && media.Schema.Value != nil && media.Schema.Value.Format == schemaFormatBinary {
			return true
		}
	}
	return false
}

// bodyLimits caps request bodies before the validator reads them. Signed-in uploads also get
// the longer transfer deadline; anonymous ones are refused by requireAuth before any read.
// Webhooks are capped as they are read, so the handler can record the refusal.
func bodyLimits(raw rawRoutes) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limit, tooLarge, early := int64(maxJSONBodyBytes), errRequestTooLarge, true
			if route, ok := raw.match(r); ok {
				limit, early = maxWebhookBodyBytes, false
				if route.upload {
					limit, tooLarge, early = maxUploadBodyBytes, errUploadTooLarge, true
					if _, ok := auth.PrincipalFrom(r.Context()); ok {
						extendDeadlines(w, true)
					}
				}
			}
			if early && r.ContentLength > limit {
				writeAppError(w, r, tooLarge())
				return
			}
			if r.Body != nil && r.Body != http.NoBody {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// downloadDeadline gives file downloads the longer write deadline.
func downloadDeadline(binary map[string]bool) gen.StrictMiddlewareFunc {
	return func(f gen.StrictHandlerFunc, operationID string) gen.StrictHandlerFunc {
		if !binary[operationID] {
			return f
		}
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, req any) (any, error) {
			extendDeadlines(w, false)
			return f(ctx, w, r, req)
		}
	}
}

// extendDeadlines moves the connection's write (and optionally read) deadline. Test
// recorders don't support deadlines, which is fine.
func extendDeadlines(w http.ResponseWriter, read bool) {
	rc := http.NewResponseController(w)
	deadline := time.Now().Add(transferTimeout)
	if read {
		_ = rc.SetReadDeadline(deadline)
	}
	_ = rc.SetWriteDeadline(deadline)
}

func errRequestTooLarge() *apperr.Error {
	return apperr.TooLarge(apperr.CodeRequestTooLarge, "The request is too large", "")
}

func errUploadTooLarge() *apperr.Error {
	return apperr.TooLarge(apperr.CodeAttachmentTooLarge, "The file is too large", "")
}

// tooLargeError reports whether err came from a body over its limit.
func tooLargeError(err error) bool {
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}
