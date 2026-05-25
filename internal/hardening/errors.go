package hardening

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
)

type ErrorType string

const (
	ErrorRateLimited     ErrorType = "rate_limited"
	ErrorServerError     ErrorType = "server_error"
	ErrorTimeout         ErrorType = "timeout"
	ErrorNetwork         ErrorType = "network_error"
	ErrorBadRequest      ErrorType = "bad_request"
	ErrorUnauthorized    ErrorType = "unauthorized"
	ErrorNotFound        ErrorType = "not_found"
	ErrorInvalidSymbol   ErrorType = "invalid_symbol"
	ErrorInvalidResponse ErrorType = "invalid_response"
	ErrorNormalizer      ErrorType = "normalizer_error"
	ErrorUnknown         ErrorType = "unknown"
)

type Classification struct {
	Type        ErrorType
	Recoverable bool
	RateLimited bool
}

func ClassifyHTTPStatus(statusCode int, body []byte) Classification {
	switch {
	case statusCode == http.StatusTooManyRequests:
		return Classification{Type: ErrorRateLimited, Recoverable: true, RateLimited: true}
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return Classification{Type: ErrorUnauthorized, Recoverable: false}
	case statusCode == http.StatusNotFound:
		return Classification{Type: ErrorNotFound, Recoverable: false}
	case statusCode == http.StatusBadRequest:
		if looksLikeInvalidSymbol(body) {
			return Classification{Type: ErrorInvalidSymbol, Recoverable: false}
		}
		return Classification{Type: ErrorBadRequest, Recoverable: false}
	case statusCode >= 500:
		return Classification{Type: ErrorServerError, Recoverable: true}
	case statusCode >= 400:
		return Classification{Type: ErrorBadRequest, Recoverable: false}
	default:
		return Classification{Type: ErrorUnknown, Recoverable: false}
	}
}

func ClassifyError(err error) Classification {
	if err == nil {
		return Classification{Type: ErrorUnknown}
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return Classification{Type: ErrorTimeout, Recoverable: true}
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return Classification{Type: ErrorTimeout, Recoverable: true}
		}
		return Classification{Type: ErrorNetwork, Recoverable: true}
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "rate limit"), strings.Contains(message, "too many requests"):
		return Classification{Type: ErrorRateLimited, Recoverable: true, RateLimited: true}
	case strings.Contains(message, "invalid symbol"), strings.Contains(message, "unknown symbol"):
		return Classification{Type: ErrorInvalidSymbol, Recoverable: false}
	case strings.Contains(message, "unauthorized"), strings.Contains(message, "forbidden"):
		return Classification{Type: ErrorUnauthorized, Recoverable: false}
	case strings.Contains(message, "not found"):
		return Classification{Type: ErrorNotFound, Recoverable: false}
	case strings.Contains(message, "unresolved request placeholder"), strings.Contains(message, "malformed template"):
		return Classification{Type: ErrorBadRequest, Recoverable: false}
	case strings.Contains(message, "normalize"), strings.Contains(message, "normalizer"):
		return Classification{Type: ErrorNormalizer, Recoverable: false}
	case strings.Contains(message, "parse"), strings.Contains(message, "invalid response"):
		return Classification{Type: ErrorInvalidResponse, Recoverable: false}
	default:
		return Classification{Type: ErrorUnknown, Recoverable: true}
	}
}

func looksLikeInvalidSymbol(body []byte) bool {
	text := strings.ToLower(string(body))
	return strings.Contains(text, "invalid symbol") ||
		strings.Contains(text, "unknown symbol") ||
		strings.Contains(text, "instrument_id") ||
		strings.Contains(text, "symbol invalid")
}
