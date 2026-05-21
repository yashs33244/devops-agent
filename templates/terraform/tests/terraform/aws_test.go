package test

import (
	"os"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAWSModuleOutputs validates that the AWS Terraform module produces the
// expected outputs when run against LocalStack. Set the LOCALSTACK_ENDPOINT
// environment variable (default http://localhost:4566) before running.
//
// Usage:
//
//	LOCALSTACK_ENDPOINT=http://localhost:4566 go test -v -run TestAWSModuleOutputs -timeout 30m
func TestAWSModuleOutputs(t *testing.T) {
	t.Parallel()

	localstackEndpoint := os.Getenv("LOCALSTACK_ENDPOINT")
	if localstackEndpoint == "" {
		localstackEndpoint = "http://localhost:4566"
	}

	options := &terraform.Options{
		TerraformDir: "../../aws",

		Vars: map[string]interface{}{
			"service_name": "test-svc",
			"environment":  "dev",
			"region":       "us-east-1",
			"cluster_name": "test-svc-dev",
			"node_min":     1,
			"node_max":     3,
			"enable_rds":   false,
		},

		EnvVars: map[string]string{
			"AWS_ACCESS_KEY_ID":     "test",
			"AWS_SECRET_ACCESS_KEY": "test",
			"AWS_DEFAULT_REGION":    "us-east-1",
		},

		// Retry plan/apply on known transient errors
		MaxRetries:         3,
		TimeBetweenRetries: 5,
	}

	defer terraform.Destroy(t, options)

	terraform.InitAndPlan(t, options)

	clusterName := terraform.Output(t, options, "cluster_name")
	require.NotEmpty(t, clusterName, "cluster_name output must not be empty")
	assert.Contains(t, clusterName, "test-svc", "cluster_name should contain the service_name")

	ecrURL := terraform.Output(t, options, "ecr_repository_url")
	require.NotEmpty(t, ecrURL, "ecr_repository_url output must not be empty")
	assert.Contains(t, ecrURL, "test-svc", "ecr_repository_url should contain the service_name")

	kubeconfigCmd := terraform.Output(t, options, "kubeconfig_command")
	require.NotEmpty(t, kubeconfigCmd, "kubeconfig_command output must not be empty")
	assert.Contains(t, kubeconfigCmd, "us-east-1", "kubeconfig_command should contain the region")

	appRoleARN := terraform.Output(t, options, "app_role_arn")
	require.NotEmpty(t, appRoleARN, "app_role_arn output must not be empty")
	assert.Contains(t, appRoleARN, "test-svc-dev-app", "app_role_arn should reference the expected role name")

	secretARN := terraform.Output(t, options, "secret_arn")
	require.NotEmpty(t, secretARN, "secret_arn output must not be empty")
}

// TestAWSValidation verifies that the AWS module rejects invalid variable values
// without reaching the cloud.
func TestAWSValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		vars      map[string]interface{}
		expectErr string
	}{
		{
			name: "invalid environment",
			vars: map[string]interface{}{
				"service_name": "test-svc",
				"environment":  "production", // not in allowed list
				"region":       "us-east-1",
				"cluster_name": "test-svc-dev",
			},
			expectErr: "environment must be one of",
		},
		{
			name: "invalid service_name with uppercase",
			vars: map[string]interface{}{
				"service_name": "TestSvc", // uppercase not allowed
				"environment":  "dev",
				"region":       "us-east-1",
				"cluster_name": "test-svc-dev",
			},
			expectErr: "service_name must be lowercase",
		},
		{
			name: "invalid region",
			vars: map[string]interface{}{
				"service_name": "test-svc",
				"environment":  "dev",
				"region":       "us-fake-9", // not a real region
				"cluster_name": "test-svc-dev",
			},
			expectErr: "must be a valid AWS region",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			options := &terraform.Options{
				TerraformDir: "../../aws",
				Vars:         tc.vars,
				EnvVars: map[string]string{
					"AWS_ACCESS_KEY_ID":     "test",
					"AWS_SECRET_ACCESS_KEY": "test",
					"AWS_DEFAULT_REGION":    "us-east-1",
				},
			}

			_, err := terraform.InitAndPlanE(t, options)
			require.Error(t, err, "expected plan to fail for case: %s", tc.name)
			assert.Contains(t, err.Error(), tc.expectErr)
		})
	}
}
