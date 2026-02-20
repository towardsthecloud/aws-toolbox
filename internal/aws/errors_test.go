package aws

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/smithy-go"
)

func TestClassifyErrorAPIKinds(t *testing.T) {
	tests := []struct {
		name string
		code string
		want ErrorKind
	}{
		{name: "access denied", code: "AccessDeniedException", want: ErrorKindAccessDenied},
		{name: "not found", code: "ResourceNotFoundException", want: ErrorKindNotFound},
		{name: "throttled", code: "ThrottlingException", want: ErrorKindThrottled},
		{name: "validation", code: "ValidationException", want: ErrorKindValidation},
		{name: "unknown", code: "InternalFailure", want: ErrorKindUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := fmt.Errorf("wrapped: %w", &smithy.GenericAPIError{Code: tc.code, Message: "msg"})
			classified := ClassifyError(err)
			if classified.Kind != tc.want {
				t.Fatalf("kind mismatch for %q: want %q, got %q", tc.code, tc.want, classified.Kind)
			}
			if classified.Code != tc.code {
				t.Fatalf("code mismatch: want %q, got %q", tc.code, classified.Code)
			}
		})
	}
}

func TestClassifyErrorTimeout(t *testing.T) {
	classified := ClassifyError(context.DeadlineExceeded)
	if classified.Kind != ErrorKindTimeout {
		t.Fatalf("expected timeout kind, got %q", classified.Kind)
	}
}

func TestClassifyErrorUnknownAndNil(t *testing.T) {
	unknown := ClassifyError(errors.New("random"))
	if unknown.Kind != ErrorKindUnknown {
		t.Fatalf("expected unknown kind, got %q", unknown.Kind)
	}

	nilErr := ClassifyError(nil)
	if nilErr.Kind != ErrorKindUnknown {
		t.Fatalf("expected unknown kind for nil, got %q", nilErr.Kind)
	}
}

func TestFormatUserError(t *testing.T) {
	err := &smithy.GenericAPIError{Code: "ValidationException", Message: "bad request"}
	message := FormatUserError(err)
	if message != "bad request (ValidationException)" {
		t.Fatalf("unexpected message: %q", message)
	}
}

func TestClassifiedErrorMethods(t *testing.T) {
	baseErr := errors.New("boom")
	withMessage := ClassifiedError{
		Kind:    ErrorKindUnknown,
		Code:    "X",
		Message: "friendly",
		Err:     baseErr,
	}
	if got := withMessage.Error(); got != "friendly" {
		t.Fatalf("unexpected message error() value: %q", got)
	}
	if !errors.Is(withMessage, baseErr) {
		t.Fatal("expected ClassifiedError to unwrap underlying error")
	}

	withErrOnly := ClassifiedError{Kind: ErrorKindUnknown, Err: baseErr}
	if got := withErrOnly.Error(); got != "boom" {
		t.Fatalf("unexpected err-only error() value: %q", got)
	}

	kindOnly := ClassifiedError{Kind: ErrorKindValidation}
	if got := kindOnly.Error(); got != string(ErrorKindValidation) {
		t.Fatalf("unexpected kind-only error() value: %q", got)
	}
}

func TestFormatUserErrorWithoutCode(t *testing.T) {
	if got := FormatUserError(nil); got != "" {
		t.Fatalf("expected empty message for nil input, got %q", got)
	}
}
