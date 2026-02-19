package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/smithy-go"
)

// ErrorKind is a normalized category for AWS/API failures.
type ErrorKind string

const (
	ErrorKindAccessDenied ErrorKind = "access_denied"
	ErrorKindNotFound     ErrorKind = "not_found"
	ErrorKindThrottled    ErrorKind = "throttled"
	ErrorKindValidation   ErrorKind = "validation"
	ErrorKindTimeout      ErrorKind = "timeout"
	ErrorKindUnknown      ErrorKind = "unknown"
)

// ClassifiedError wraps an error with a normalized category and service details.
type ClassifiedError struct {
	Kind    ErrorKind
	Code    string
	Message string
	Err     error
}

func (e ClassifiedError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Kind)
}

func (e ClassifiedError) Unwrap() error {
	return e.Err
}

// ClassifyError maps context and AWS smithy API errors into normalized categories.
func ClassifyError(err error) ClassifiedError {
	if err == nil {
		return ClassifiedError{Kind: ErrorKindUnknown}
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return ClassifiedError{
			Kind:    ErrorKindTimeout,
			Code:    "Timeout",
			Message: "request timed out before AWS returned a response",
			Err:     err,
		}
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		message := apiErr.ErrorMessage()
		kind := classifyByCode(code)
		if message == "" {
			message = err.Error()
		}

		return ClassifiedError{
			Kind:    kind,
			Code:    code,
			Message: message,
			Err:     err,
		}
	}

	return ClassifiedError{
		Kind:    ErrorKindUnknown,
		Code:    "UnknownError",
		Message: err.Error(),
		Err:     err,
	}
}

// FormatUserError returns a human-friendly AWS error string.
func FormatUserError(err error) string {
	classified := ClassifyError(err)
	if classified.Code != "" {
		return fmt.Sprintf("%s (%s)", classified.Message, classified.Code)
	}
	return classified.Message
}

func classifyByCode(code string) ErrorKind {
	lower := strings.ToLower(code)
	switch {
	case strings.Contains(lower, "accessdenied"), strings.Contains(lower, "unauthorized"):
		return ErrorKindAccessDenied
	case strings.Contains(lower, "notfound"), strings.Contains(lower, "nosuch"):
		return ErrorKindNotFound
	case strings.Contains(lower, "throttl"), strings.Contains(lower, "toomanyrequests"):
		return ErrorKindThrottled
	case strings.Contains(lower, "validation"), strings.Contains(lower, "invalid"):
		return ErrorKindValidation
	default:
		return ErrorKindUnknown
	}
}
