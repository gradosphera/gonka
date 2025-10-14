package testenv

import (
	"os"
	"sync"
)

var (
	isTestNet bool
	once      sync.Once
)

// IsTestNet returns true if IS_TEST_NET environment variable is explicitly set to "true".
// The value is read only once on first call and cached for subsequent calls.
func IsTestNet() bool {
	once.Do(func() {
		isTestNet = os.Getenv("IS_TEST_NET") == "true"
	})
	return isTestNet
}

