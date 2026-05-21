package objects_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"

	"github.com/nightshiftco/nightshift/internal/objects"
)

// newFakeS3 boots an in-memory S3-compatible server (gofakes3) and
// returns an [objects.S3Backend] pointed at it. Examples use this so
// they can run with no external dependencies. The returned cleanup
// shuts the server down.
func newFakeS3() (*objects.S3Backend, func()) {
	const bucket, ak, sk = "nightshift", "test-ak", "test-sk"

	srv := httptest.NewServer(gofakes3.New(s3mem.New()).Server())

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
		srv.Close()
		panic(err)
	}

	b, err := objects.NewS3(context.Background(), objects.S3Config{
		Endpoint:        srv.URL,
		Region:          "us-east-1",
		Bucket:          bucket,
		AccessKeyID:     ak,
		SecretAccessKey: sk,
		UsePathStyle:    true,
	})
	if err != nil {
		srv.Close()
		panic(err)
	}
	return b, srv.Close
}

// Example_quickstart shows the in-band PutBytes shortcut: a single call
// uploads bytes and lands the object in [objects.UploadStateReady]. Use
// this when the API process already has the bytes in memory and wants
// to avoid the Initiate/PUT/Finalize round trip.
func Example_quickstart() {
	store, cleanup := newFakeS3()
	defer cleanup()

	ctx := context.Background()
	obj, err := store.PutBytes(ctx, "artifacts", "abc/report.txt",
		"text/plain", []byte("hello, nightshift"))
	if err != nil {
		fmt.Println("put:", err)
		return
	}
	fmt.Println("size:", obj.SizeBytes)
	fmt.Println("state ready:", obj.State == objects.UploadStateReady)
	// Output:
	// size: 17
	// state ready: true
}

// ExampleS3Backend_Initiate walks the three-step upload lifecycle that
// out-of-process clients use: the API returns a presigned URL, the
// client PUTs bytes directly to S3, and the API flips the object to
// READY with [objects.S3Backend.Finalize].
func ExampleS3Backend_Initiate() {
	store, cleanup := newFakeS3()
	defer cleanup()

	ctx := context.Background()

	obj, uploadURL, headers, _, err := store.Initiate(ctx, objects.InitiateSpec{
		Bucket:      "artifacts",
		Key:         "abc/report.txt",
		ContentType: "text/plain",
	})
	if err != nil {
		fmt.Println("initiate:", err)
		return
	}
	fmt.Println("pending:", obj.State == objects.UploadStatePending)

	body := []byte("hello, nightshift")
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("upload:", err)
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	final, err := store.Finalize(ctx, "artifacts", "abc/report.txt")
	if err != nil {
		fmt.Println("finalize:", err)
		return
	}
	fmt.Println("ready:", final.State == objects.UploadStateReady)
	fmt.Println("size:", final.SizeBytes)
	// Output:
	// pending: true
	// ready: true
	// size: 17
}

// ExampleS3Backend_DownloadURL returns a presigned GET URL the caller
// can fetch directly. The API never proxies bytes — it only mints
// short-lived URLs.
func ExampleS3Backend_DownloadURL() {
	store, cleanup := newFakeS3()
	defer cleanup()

	ctx := context.Background()
	if _, err := store.PutBytes(ctx, "artifacts", "abc/report.txt",
		"text/plain", []byte("hello, nightshift")); err != nil {
		fmt.Println("put:", err)
		return
	}

	url, _, err := store.DownloadURL(ctx, "artifacts", "abc/report.txt", 0)
	if err != nil {
		fmt.Println("download url:", err)
		return
	}

	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("get:", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
	// Output:
	// hello, nightshift
}
