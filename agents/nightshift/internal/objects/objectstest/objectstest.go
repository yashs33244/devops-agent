// Package objectstest provides a shared in-process ObjectStore for
// tests in other packages. The implementation is gofakes3 + S3Backend
// — same shape that internal/objects's compliance suite uses, so
// service-level tests exercise the same backend the integration tests
// hit, just without a real MinIO container.
package objectstest

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"

	"github.com/nightshiftco/nightshift/internal/objects"
)

// New boots a gofakes3 in-memory S3 server, pre-creates a bucket, and
// returns an ObjectStore pointed at it. Cleanup (server shutdown) is
// registered via t.Cleanup.
func New(t *testing.T) objects.ObjectStore {
	t.Helper()
	mem := s3mem.New()
	faker := gofakes3.New(mem)
	srv := httptest.NewServer(faker.Server())
	t.Cleanup(srv.Close)

	const bucket = "nightshift"
	const ak, sk = "test-ak", "test-sk"

	bootstrap := s3.NewFromConfig(aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider(ak, sk, ""),
	}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(srv.URL)
		o.UsePathStyle = true
	})
	if _, err := bootstrap.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	store, err := objects.NewS3(context.Background(), objects.S3Config{
		Endpoint:        srv.URL,
		Region:          "us-east-1",
		Bucket:          bucket,
		AccessKeyID:     ak,
		SecretAccessKey: sk,
		UsePathStyle:    true,
	})
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}
	return store
}
