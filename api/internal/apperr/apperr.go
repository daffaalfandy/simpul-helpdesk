// Package apperr defines errors that map to HTTP problem responses.
package apperr

import (
	"fmt"
	"net/http"
	"time"
)

// Stable error codes returned in problem responses. Clients branch on these.
const (
	CodeUnauthenticated    = "unauthenticated"
	CodeInvalidCredentials = "invalid_credentials"
	CodeForbidden          = "forbidden"
	CodeNotFound           = "not_found"
	CodeMethodNotAllowed   = "method_not_allowed"
	CodeCrossOrigin        = "cross_origin"
	CodeRateLimited        = "rate_limited"
	CodeInvalidRequest     = "invalid_request"
	CodeInvalidToken       = "invalid_token"
	CodeWeakPassword       = "weak_password"
	CodeInvalidEmail       = "invalid_email"
	CodeInvalidName        = "invalid_name"
	CodeInvalidRole        = "invalid_role"
	CodeClientRequired     = "client_required"
	CodeClientNotAllowed   = "client_not_allowed"
	CodeClientNotFound     = "client_not_found"
	CodeInternal           = "internal"

	CodeClientNameTaken       = "client_name_taken"
	CodeClientInUse           = "client_in_use"
	CodeInvalidContractPeriod = "invalid_contract_period"
	CodeInvalidAgent          = "invalid_agent"
	CodeSlaPlanNotFound       = "sla_plan_not_found"
	CodeSlaPlanNameTaken      = "sla_plan_name_taken"
	CodeSlaPlanInUse          = "sla_plan_in_use"
	CodeSlaPlanIsDefault      = "sla_plan_is_default"
	CodeInvalidSlaTargets     = "invalid_sla_targets"
	CodeInvalidBusinessHours  = "invalid_business_hours"
	CodeHolidayDateTaken      = "holiday_date_taken"
)

// Error is a domain error with a stable machine-readable code.
type Error struct {
	Status     int
	Code       string
	Title      string
	Detail     string
	RetryAfter time.Duration
	Fields     []FieldError
	Err        error
}

// FieldError describes a problem with one request field.
type FieldError struct {
	Field   string
	Message string
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Err)
	}
	return e.Code
}

func (e *Error) Unwrap() error { return e.Err }

// Unauthenticated means the request has no valid session.
func Unauthenticated() *Error {
	return &Error{Status: http.StatusUnauthorized, Code: CodeUnauthenticated, Title: "Sign in required"}
}

// InvalidCredentials means the email or password is wrong. It never says which.
func InvalidCredentials() *Error {
	return &Error{Status: http.StatusUnauthorized, Code: CodeInvalidCredentials, Title: "Wrong email or password"}
}

// Forbidden means the user is signed in but may not do this.
func Forbidden() *Error {
	return &Error{Status: http.StatusForbidden, Code: CodeForbidden, Title: "You don't have access to this"}
}

// NotFound means the resource does not exist (or is hidden from the user).
func NotFound() *Error {
	return &Error{Status: http.StatusNotFound, Code: CodeNotFound, Title: "Not found"}
}

// MethodNotAllowed means the path exists but not for this HTTP method.
func MethodNotAllowed() *Error {
	return &Error{Status: http.StatusMethodNotAllowed, Code: CodeMethodNotAllowed, Title: "Method not allowed"}
}

// CrossOrigin means a state-changing request came from another site.
func CrossOrigin() *Error {
	return &Error{Status: http.StatusForbidden, Code: CodeCrossOrigin, Title: "Cross-origin request blocked"}
}

// RateLimited means too many attempts; retry after the given duration.
func RateLimited(retryAfter time.Duration) *Error {
	return &Error{Status: http.StatusTooManyRequests, Code: CodeRateLimited, Title: "Too many attempts", RetryAfter: retryAfter}
}

// Internal hides an unexpected error behind a generic 500.
func Internal() *Error {
	return &Error{Status: http.StatusInternalServerError, Code: CodeInternal, Title: "Something went wrong on our side"}
}

// Invalid is a 400 with a specific code.
func Invalid(code, title, detail string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: code, Title: title, Detail: detail}
}

// Conflict is a 409: the request clashes with the current state, e.g. a taken name.
func Conflict(code, title, detail string) *Error {
	return &Error{Status: http.StatusConflict, Code: code, Title: title, Detail: detail}
}

// InvalidRequest means the request doesn't match the API contract.
func InvalidRequest(detail string) *Error {
	return Invalid(CodeInvalidRequest, "The request is not valid", detail)
}

// InvalidToken means a reset or action token is unknown, used or expired.
func InvalidToken() *Error {
	return Invalid(CodeInvalidToken, "This link is invalid or has expired", "")
}

// WeakPassword means the new password does not meet the policy.
func WeakPassword(detail string) *Error {
	return Invalid(CodeWeakPassword, "Choose a stronger password", detail)
}
