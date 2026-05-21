package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cloudDirs lists the cloud provider directories to validate.
var cloudDirs = []string{"aws", "azure", "gcp"}

// TestTerraformFmt checks that all HCL files across the three cloud modules
// are properly formatted (terraform fmt -check -recursive).
func TestTerraformFmt(t *testing.T) {
	t.Parallel()

	// Resolve the repo root relative to this test file
	repoRoot, err := filepath.Abs("../../")
	require.NoError(t, err, "could not resolve repo root")

	for _, cloud := range cloudDirs {
		cloud := cloud
		t.Run(cloud, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(repoRoot, cloud)
			_, err := os.Stat(dir)
			require.NoError(t, err, "directory %s does not exist", dir)

			cmd := exec.Command("terraform", "fmt", "-check", "-recursive", dir)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()

			assert.NoError(t, err,
				"terraform fmt check failed for %s:\n%s\nRun `terraform fmt -recursive %s` to fix.",
				cloud, string(out), dir,
			)
		})
	}
}

// TestTerraformValidate runs `terraform init -backend=false` followed by
// `terraform validate` for each cloud module and asserts exit code 0.
func TestTerraformValidate(t *testing.T) {
	t.Parallel()

	repoRoot, err := filepath.Abs("../../")
	require.NoError(t, err, "could not resolve repo root")

	for _, cloud := range cloudDirs {
		cloud := cloud
		t.Run(cloud, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(repoRoot, cloud)
			_, err := os.Stat(dir)
			require.NoError(t, err, "directory %s does not exist", dir)

			// Init without configuring a backend so we do not need real credentials
			initCmd := exec.Command("terraform", "init", "-backend=false", "-input=false")
			initCmd.Dir = dir
			initOut, err := initCmd.CombinedOutput()
			require.NoError(t, err,
				"terraform init failed for %s:\n%s", cloud, string(initOut))

			// Validate the configuration
			validateCmd := exec.Command("terraform", "validate", "-json")
			validateCmd.Dir = dir
			validateOut, err := validateCmd.CombinedOutput()

			assert.NoError(t, err,
				"terraform validate failed for %s:\n%s", cloud, string(validateOut))
		})
	}
}

// TestTerraformNoHardcodedSecrets is a static analysis guard that ensures
// none of the .tf files contain obviously hardcoded credentials.
func TestTerraformNoHardcodedSecrets(t *testing.T) {
	t.Parallel()

	repoRoot, err := filepath.Abs("../../")
	require.NoError(t, err)

	// Patterns that should never appear in committed .tf files
	forbidden := []string{
		"aws_secret_access_key",
		"aws_access_key_id     =",
		"password              = \"[^\"]{8,}\"", // non-placeholder passwords
	}

	for _, cloud := range cloudDirs {
		dir := filepath.Join(repoRoot, cloud)

		matches, err := filepath.Glob(filepath.Join(dir, "*.tf"))
		require.NoError(t, err)

		for _, tfFile := range matches {
			content, err := os.ReadFile(tfFile)
			require.NoError(t, err)

			for _, pattern := range forbidden {
				grepCmd := exec.Command("grep", "-En", pattern, tfFile)
				out, _ := grepCmd.Output()
				assert.Empty(t, string(out),
					fmt.Sprintf("potential hardcoded secret found in %s matching pattern %q", tfFile, pattern),
				)
				_ = content // kept to avoid import error if grep is unavailable
			}
		}
	}
}
