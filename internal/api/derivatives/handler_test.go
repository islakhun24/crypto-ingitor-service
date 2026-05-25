package derivatives

import (
	"context"
	"net/http"
	"testing"
)

func TestErrorStatusUsesStableCodes(t *testing.T) {
	status, code := errorStatus(ErrNotFound)
	if status != http.StatusNotFound || code != "not_found" {
		t.Fatalf("not found status/code = %d/%s", status, code)
	}

	status, code = errorStatus(context.DeadlineExceeded)
	if status != http.StatusGatewayTimeout || code != "timeout" {
		t.Fatalf("timeout status/code = %d/%s", status, code)
	}
}
