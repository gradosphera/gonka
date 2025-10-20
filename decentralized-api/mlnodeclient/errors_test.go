package mlnodeclient

import (
	"errors"
	"net/http"
	"testing"
)

func TestErrAPINotImplemented(t *testing.T) {
	t.Run("error message format", func(t *testing.T) {
		err := NewAPINotImplementedError("/api/v1/test", http.StatusNotFound)
		expected := "API endpoint not implemented: /api/v1/test (HTTP 404)"
		if err.Error() != expected {
			t.Errorf("expected error message %q, got %q", expected, err.Error())
		}
	})

	t.Run("errors.Is comparison", func(t *testing.T) {
		err1 := NewAPINotImplementedError("/api/v1/test", http.StatusNotFound)
		err2 := &ErrAPINotImplemented{}

		if !errors.Is(err1, err2) {
			t.Error("expected errors.Is to return true for ErrAPINotImplemented types")
		}
	})

	t.Run("type assertion", func(t *testing.T) {
		err := NewAPINotImplementedError("/api/v1/test", http.StatusNotFound)

		apiErr, ok := err.(*ErrAPINotImplemented)
		if !ok {
			t.Fatal("expected type assertion to succeed")
		}

		if apiErr.Endpoint != "/api/v1/test" {
			t.Errorf("expected endpoint /api/v1/test, got %s", apiErr.Endpoint)
		}
		if apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("expected status code 404, got %d", apiErr.StatusCode)
		}
	})

	t.Run("different status codes", func(t *testing.T) {
		err404 := NewAPINotImplementedError("/api/v1/test", http.StatusNotFound)
		err405 := NewAPINotImplementedError("/api/v1/test", http.StatusMethodNotAllowed)

		apiErr404 := err404.(*ErrAPINotImplemented)
		apiErr405 := err405.(*ErrAPINotImplemented)

		if apiErr404.StatusCode != 404 {
			t.Errorf("expected status code 404, got %d", apiErr404.StatusCode)
		}
		if apiErr405.StatusCode != 405 {
			t.Errorf("expected status code 405, got %d", apiErr405.StatusCode)
		}
	})

	t.Run("not equal to generic error", func(t *testing.T) {
		apiErr := NewAPINotImplementedError("/api/v1/test", http.StatusNotFound)
		genericErr := errors.New("some error")

		if errors.Is(genericErr, apiErr) {
			t.Error("expected errors.Is to return false for different error types")
		}
	})
}
