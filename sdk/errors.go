package configclient

import (
	"fmt"
	"strings"
)

// MissingKeysError is returned when required config keys are not found.
type MissingKeysError struct {
	Keys []string
}

func (e *MissingKeysError) Error() string {
	return fmt.Sprintf("configclient: missing required config keys: %s", strings.Join(e.Keys, ", "))
}
