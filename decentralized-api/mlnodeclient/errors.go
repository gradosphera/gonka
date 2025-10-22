package mlnodeclient

import "fmt"

// ErrAPINotImplemented indicates that the ML node doesn't support this API endpoint.
// This typically happens when an older version of the ML node is running.
// Can be checked with errors.Is(err, ErrAPINotImplemented)
type ErrAPINotImplemented struct {
	Endpoint   string
	StatusCode int
}

func (e *ErrAPINotImplemented) Error() string {
	return fmt.Sprintf("API endpoint not implemented: %s (HTTP %d)", e.Endpoint, e.StatusCode)
}

func (e *ErrAPINotImplemented) Is(target error) bool {
	_, ok := target.(*ErrAPINotImplemented)
	return ok
}

// NewAPINotImplementedError creates a new ErrAPINotImplemented error
func NewAPINotImplementedError(endpoint string, statusCode int) error {
	return &ErrAPINotImplemented{
		Endpoint:   endpoint,
		StatusCode: statusCode,
	}
}
