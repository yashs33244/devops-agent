package storage

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/objects/objectstest"
	"github.com/nightshiftco/nightshift/internal/records"
)

func newService(t *testing.T) *Service {
	t.Helper()
	rec, err := records.OpenSQLite("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rec.Close() })
	return &Service{Records: rec, Objects: objectstest.New(t)}
}

func TestPutGetDelete(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	put, err := s.PutRecord(ctx, &nsv1.PutRecordRequest{
		Collection:  "runs",
		Key:         "r1",
		Attributes:  map[string]string{"user_id": "alice"},
		Data:        []byte(`{"x":1}`),
		ContentType: "application/json",
	})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if put.Record.Version != 1 {
		t.Fatalf("version=%d", put.Record.Version)
	}

	got, err := s.GetRecord(ctx, &nsv1.GetRecordRequest{Collection: "runs", Key: "r1"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got.Record.Data) != `{"x":1}` {
		t.Fatalf("data=%q", got.Record.Data)
	}

	if _, err := s.DeleteRecord(ctx, &nsv1.DeleteRecordRequest{Collection: "runs", Key: "r1"}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = s.GetRecord(ctx, &nsv1.GetRecordRequest{Collection: "runs", Key: "r1"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestObjectFlow(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	init, err := s.InitiateObjectUpload(ctx, &nsv1.InitiateObjectUploadRequest{
		Bucket:      "bk",
		Key:         "a/b.txt",
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if init.UploadUrl == "" {
		t.Fatalf("no upload url")
	}

	body := "hello"
	req, _ := http.NewRequest(http.MethodPut, init.UploadUrl, bytes.NewBufferString(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("PUT status=%d", resp.StatusCode)
	}

	fin, err := s.FinalizeObjectUpload(ctx, &nsv1.FinalizeObjectUploadRequest{Bucket: "bk", Key: "a/b.txt"})
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if fin.Object.UploadState != nsv1.UploadState_UPLOAD_STATE_READY {
		t.Fatalf("state=%v", fin.Object.UploadState)
	}
	if fin.Object.SizeBytes != int64(len(body)) {
		t.Fatalf("size=%d", fin.Object.SizeBytes)
	}

	dl, err := s.GetObjectDownloadURL(ctx, &nsv1.GetObjectDownloadURLRequest{Bucket: "bk", Key: "a/b.txt"})
	if err != nil {
		t.Fatalf("dl url: %v", err)
	}
	if dl.DownloadUrl == "" {
		t.Fatalf("no download url")
	}
}
