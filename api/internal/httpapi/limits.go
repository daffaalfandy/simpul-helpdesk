package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// Uploads carry one file plus multipart framing. Transfers get their own deadline because the
// server-wide read and write timeouts would cut a 10 MB transfer on a slow link.
const (
	maxJSONBodyBytes       = 1 << 20  // 1 MiB
	multipartOverheadBytes = 64 << 10 // part headers and boundaries
	maxUploadBodyBytes     = tickets.MaxAttachmentBytes + multipartOverheadBytes
	transferTimeout        = 10 * time.Minute

	contentTypeMultipart    = "multipart/form-data"
	contentTypeOctetStream  = "application/octet-stream"
	pathParamOpen           = "{"
	routeKeySeparator       = " "
	successStatusClassFirst = "2"
)

// routeKey identifies a request by method and path, e.g. "POST /api/v1/attachments".
func routeKey(method, path string) string { return method + routeKeySeparator + path }

// streamedRoutes lists the operations with a multipart body, read from the spec. They skip the
// validator, which reads whole bodies into memory, and their handlers check the request.
func streamedRoutes(spec *openapi3.T) map[string]bool {
	out := map[string]bool{}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op.RequestBody == nil || op.RequestBody.Value == nil ||
				op.RequestBody.Value.Content.Get(contentTypeMultipart) == nil {
				continue
			}
			if strings.Contains(path, pathParamOpen) {
				panic(fmt.Sprintf("streamed route %s %s has a path parameter; routeKey can't match it", method, path))
			}
			out[routeKey(method, apiPrefix+path)] = true
		}
	}
	return out
}

// binaryResponseOps lists the operations (Go names) that answer with a file.
func binaryResponseOps(spec *openapi3.T) map[string]bool {
	out := map[string]bool{}
	for _, item := range spec.Paths.Map() {
		for _, op := range item.Operations() {
			for status, resp := range op.Responses.Map() {
				if strings.HasPrefix(status, successStatusClassFirst) && resp.Value != nil &&
					resp.Value.Content.Get(contentTypeOctetStream) != nil {
					out[goName(op.OperationID)] = true
				}
			}
		}
	}
	return out
}

// bodyLimits caps request bodies before the validator reads them. Signed-in uploads also get
// the longer transfer deadline; anonymous ones are refused by requireAuth before any read.
func bodyLimits(streamed map[string]bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limit, tooLarge := int64(maxJSONBodyBytes), errRequestTooLarge
			if streamed[routeKey(r.Method, r.URL.Path)] {
				limit, tooLarge = maxUploadBodyBytes, errUploadTooLarge
				if _, ok := auth.PrincipalFrom(r.Context()); ok {
					extendDeadlines(w, true)
				}
			}
			if r.ContentLength > limit {
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
