package hardening

import (
	"context"
	"net/http"
	"testing"
)

func TestClassifyHTTPStatus(t *testing.T) {
	tests := []struct {
		status      int
		body        []byte
		wantType    ErrorType
		recoverable bool
	}{
		{status: http.StatusTooManyRequests, wantType: ErrorRateLimited, recoverable: true},
		{status: http.StatusInternalServerError, wantType: ErrorServerError, recoverable: true},
		{status: http.StatusBadRequest, body: []byte(`invalid symbol`), wantType: ErrorInvalidSymbol},
		{status: http.StatusUnauthorized, wantType: ErrorUnauthorized},
		{status: http.StatusNotFound, wantType: ErrorNotFound},
	}

	for _, test := range tests {
		got := ClassifyHTTPStatus(test.status, test.body)
		if got.Type != test.wantType || got.Recoverable != test.recoverable {
			t.Fatalf("ClassifyHTTPStatus(%d) = %+v, want %s recoverable=%t", test.status, got, test.wantType, test.recoverable)
		}
	}
}

func TestClassifyErrorTimeout(t *testing.T) {
	got := ClassifyError(context.DeadlineExceeded)
	if got.Type != ErrorTimeout || !got.Recoverable {
		t.Fatalf("ClassifyError(deadline) = %+v, want timeout recoverable", got)
	}
}
