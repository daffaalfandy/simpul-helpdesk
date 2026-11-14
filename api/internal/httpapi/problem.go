package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

// statusClientClosedRequest is nginx's code for a request the client gave up on, for
// example the board cancelling a refresh while a card moves. Nobody reads the answer.
const statusClientClosedRequest = 499

// writeProblem renders err as RFC 9457 problem details. Unknown errors become a 500
// and are logged with the request ID; a request the client cancelled is not an error.
func writeProblem(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	if errors.Is(err, context.Canceled) && r.Context().Err() != nil {
		w.WriteHeader(statusClientClosedRequest)
		return
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		log.ErrorContext(r.Context(), "unhandled error",
			"err", err, "method", r.Method, "path", r.URL.Path, "request_id", middleware.GetReqID(r.Context()))
		ae = apperr.Internal()
	}
	writeAppError(w, r, ae)
}

func writeAppError(w http.ResponseWriter, r *http.Request, ae *apperr.Error) {
	p := gen.Problem{
		Type:   problemTypeBlank,
		Title:  ae.Title,
		Status: ae.Status,
		Code:   ae.Code,
	}
	if ae.Detail != "" {
		p.Detail = &ae.Detail
	}
	if id := middleware.GetReqID(r.Context()); id != "" {
		p.RequestId = &id
	}
	if len(ae.Fields) > 0 {
		fields := make([]struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		}, len(ae.Fields))
		for i, f := range ae.Fields {
			fields[i].Field, fields[i].Message = f.Field, f.Message
		}
		p.Errors = &fields
	}
	if ae.RetryAfter > 0 {
		w.Header().Set(headerRetryAfter, strconv.Itoa(int(math.Ceil(ae.RetryAfter.Seconds()))))
	}
	w.Header().Set(headerContentType, contentTypeProblem)
	w.Header().Set(headerCacheControl, cacheNoStore)
	w.WriteHeader(ae.Status)
	_ = json.NewEncoder(w).Encode(p)
}

func problemHandler(ae func() *apperr.Error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { writeAppError(w, r, ae()) }
}
