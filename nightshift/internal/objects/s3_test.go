package objects

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

// newS3Backend boots a gofakes3 in-memory S3 server, creates a bucket,
// and returns an S3Backend pointed at it.
func newS3Backend(t *testing.T) (*S3Backend, *httptest.Server) {
	t.Helper()
	mem := s3mem.New()
	faker := gofakes3.New(mem)
	srv := httptest.NewServer(faker.Server())
	t.Cleanup(srv.Close)

	const bucket = "nightshift"
	const ak, sk = "test-ak", "test-sk"

	// Bootstrap the bucket up front (NewS3 below will HeadBucket).
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

	b, err := NewS3(context.Background(), S3Config{
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
	return b, srv
}

func newS3Store(t *testing.T) ObjectStore {
	t.Helper()
	b, _ := newS3Backend(t)
	return b
}

func TestS3Compliance(t *testing.T) {
	runObjectStoreComplianceSuite(t, newS3Store)
}

// V4 signature-tamper enforcement is verified by the integration test
// against a real MinIO (build tag minio_integration); gofakes3 does
// not validate signatures, so the in-process unit suite cannot
// exercise the failure path.

func TestS3HeadBucketFailFast(t *testing.T) {
	mem := s3mem.New()
	faker := gofakes3.New(mem)
	srv := httptest.NewServer(faker.Server())
	t.Cleanup(srv.Close)

	// Note: no CreateBucket here — NewS3 must error out cleanly.
	_, err := NewS3(context.Background(), S3Config{
		Endpoint:        srv.URL,
		Region:          "us-east-1",
		Bucket:          "missing",
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
		UsePathStyle:    true,
	})
	if err == nil {
		t.Fatal("expected NewS3 to fail with HeadBucket against missing bucket")
	}
	if !strings.Contains(err.Error(), "HeadBucket") {
		t.Fatalf("expected HeadBucket in error, got %q", err)
	}
}
