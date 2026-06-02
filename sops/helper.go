package sops

import (
	"encoding/json"
	"testing"

	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/getsops/sops/v3/decrypt"
	"github.com/stretchr/testify/assert/yaml"
)

// sopsVarLoader is a helper to load environment variables from SOPS-encrypted files.
type sopsVarLoader struct{}

// LoadEnvFromFile decrypts a SOPS file and sets its key-value pairs as environment variables for the test.
// Supported formats: JSON, YAML.
func (l *sopsVarLoader) LoadEnvFromFile(t *testing.T, fPath string) error {
	t.Helper()

	// Decrypt the file using sops
	confData, err := decrypt.File(fPath, "")
	if err != nil {
		return err
	}

	vals := map[string]string{}
	// Parse based on file extension/format
	if formats.IsYAMLFile(fPath) {
		err = yaml.Unmarshal(confData, &vals)
	} else if formats.IsJSONFile(fPath) {
		err = json.Unmarshal(confData, &vals)
	}
	if err != nil {
		return err
	}

	// Set environment variables for the duration of the test
	for k, v := range vals {
		t.Setenv(k, v)
	}

	return nil
}

// NewSOPSEnvVarLoader creates a new loader instance.
func NewSOPSEnvVarLoader() *sopsVarLoader {
	return new(sopsVarLoader)
}
