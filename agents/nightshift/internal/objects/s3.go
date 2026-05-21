package objects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// S3Config configures the S3-compatible backend. Endpoint is the URL the
// API uses for control-plane ops (HeadBucket, HeadObject, etc.).
// PresignEndpoint is the URL baked into presigned URLs handed to clients
// — the host is part of V4's canonical request and cannot be changed
// post-signing. For an in-cluster MinIO the two are equal; for a
// deployment that exposes MinIO externally, PresignEndpoint differs.
type S3Config struct {
	Endpoint        string
	PresignEndpoint string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
}

// S3Backend implements ObjectStore against any S3 v4 endpoint
// (real S3, MinIO, etc.). Logical (bucket, key) pairs map to S3 keys
// `<bucket>/<key>` inside the configured S3 bucket; metadata lives
// alongside at `<bucket>/<key>.meta.json`.
type S3Backend struct {
	cfg     S3Config
	client  *s3.Client
	presign *s3.PresignClient
}

const metaSuffix = ".meta.json"

// NewS3 constructs an S3 backend and runs HeadBucket to fail fast on
// missing bucket / bad creds.
func NewS3(ctx context.Context, cfg S3Config) (*S3Backend, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("objects: s3 endpoint required")
	}
	if cfg.Region == "" {
		return nil, errors.New("objects: s3 region required")
	}
	if cfg.Bucket == "" {
		return nil, errors.New("objects: s3 bucket required")
	}
	if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, errors.New("objects: s3 credentials required")
	}
	presignEndpoint := cfg.PresignEndpoint
	if presignEndpoint == "" {
		presignEndpoint = cfg.Endpoint
	}

	awsCfg := aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = cfg.UsePathStyle
	})

	presignClient := s3.NewPresignClient(s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(presignEndpoint)
		o.UsePathStyle = cfg.UsePathStyle
	}))

	if _, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	}); err != nil {
		return nil, fmt.Errorf("objects: s3 HeadBucket %q: %w", cfg.Bucket, err)
	}

	return &S3Backend{
		cfg:     cfg,
		client:  client,
		presign: presignClient,
	}, nil
}

// dataKey is the S3 key that stores the bytes for a logical
// (bucket, key) pair.
func dataKey(bucket, key string) string {
	return bucket + "/" + key
}

// metaKey is the S3 key that stores the JSON sidecar for a logical
// (bucket, key) pair.
func metaKey(bucket, key string) string {
	return dataKey(bucket, key) + metaSuffix
}

// s3Meta is the JSON sidecar that augments a stored S3 object with
// fields S3 itself doesn't natively express (UploadState, our
// CreatedAt/UpdatedAt, user Metadata). One sidecar per data object,
// keyed by metaKey(bucket, key).
type s3Meta struct {
	Bucket      string            `json:"bucket"`
	Key         string            `json:"key"`
	ContentType string            `json:"content_type"`
	SizeBytes   int64             `json:"size_bytes"`
	ETag        string            `json:"etag"`
	Metadata    map[string]string `json:"metadata"`
	State       UploadState       `json:"state"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

func (m *s3Meta) toObject() Object {
	return Object{
		Bucket:      m.Bucket,
		Key:         m.Key,
		ContentType: m.ContentType,
		SizeBytes:   m.SizeBytes,
		ETag:        m.ETag,
		Metadata:    m.Metadata,
		State:       m.State,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func (b *S3Backend) writeMeta(ctx context.Context, m *s3Meta) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(b.cfg.Bucket),
		Key:         aws.String(metaKey(m.Bucket, m.Key)),
		Body:        strings.NewReader(string(body)),
		ContentType: aws.String("application/json"),
	})
	return err
}

func (b *S3Backend) readMeta(ctx context.Context, bucket, key string) (*s3Meta, error) {
	out, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.cfg.Bucket),
		Key:    aws.String(metaKey(bucket, key)),
	})
	if err != nil {
		if isS3NotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer func() { _ = out.Body.Close() }()
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, err
	}
	m := &s3Meta{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (b *S3Backend) Initiate(ctx context.Context, spec InitiateSpec) (Object, string, map[string]string, time.Time, error) {
	if spec.Bucket == "" || spec.Key == "" {
		return Object{}, "", nil, time.Time{}, fmt.Errorf("objects: bucket and key required")
	}
	if spec.TTL <= 0 {
		spec.TTL = 15 * time.Minute
	}
	now := time.Now().UTC()
	m := &s3Meta{
		Bucket:      spec.Bucket,
		Key:         spec.Key,
		ContentType: spec.ContentType,
		SizeBytes:   spec.SizeBytes,
		Metadata:    spec.Metadata,
		State:       UploadStatePending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := b.writeMeta(ctx, m); err != nil {
		return Object{}, "", nil, time.Time{}, fmt.Errorf("objects: write meta: %w", err)
	}

	put := &s3.PutObjectInput{
		Bucket: aws.String(b.cfg.Bucket),
		Key:    aws.String(dataKey(spec.Bucket, spec.Key)),
	}
	if spec.ContentType != "" {
		put.ContentType = aws.String(spec.ContentType)
	}
	signed, err := b.presign.PresignPutObject(ctx, put, s3.WithPresignExpires(spec.TTL))
	if err != nil {
		return Object{}, "", nil, time.Time{}, fmt.Errorf("objects: presign put: %w", err)
	}

	headers := map[string]string{}
	if spec.ContentType != "" {
		headers["Content-Type"] = spec.ContentType
	}
	expires := now.Add(spec.TTL)
	return m.toObject(), signed.URL, headers, expires, nil
}

func (b *S3Backend) Finalize(ctx context.Context, bucket, key string) (Object, error) {
	m, err := b.readMeta(ctx, bucket, key)
	if err != nil {
		return Object{}, err
	}
	head, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.cfg.Bucket),
		Key:    aws.String(dataKey(bucket, key)),
	})
	if err != nil {
		if isS3NotFound(err) {
			return Object{}, fmt.Errorf("%w: bytes not uploaded", ErrInvalidState)
		}
		return Object{}, fmt.Errorf("objects: head object: %w", err)
	}
	if head.ContentLength != nil {
		m.SizeBytes = *head.ContentLength
	}
	if head.ETag != nil {
		m.ETag = strings.Trim(*head.ETag, `"`)
	}
	m.State = UploadStateReady
	m.UpdatedAt = time.Now().UTC()
	if err := b.writeMeta(ctx, m); err != nil {
		return Object{}, fmt.Errorf("objects: write meta: %w", err)
	}
	return m.toObject(), nil
}

func (b *S3Backend) PutBytes(ctx context.Context, bucket, key, contentType string, body []byte) (Object, error) {
	if bucket == "" || key == "" {
		return Object{}, fmt.Errorf("objects: bucket and key required")
	}
	put := &s3.PutObjectInput{
		Bucket: aws.String(b.cfg.Bucket),
		Key:    aws.String(dataKey(bucket, key)),
		Body:   strings.NewReader(string(body)),
	}
	if contentType != "" {
		put.ContentType = aws.String(contentType)
	}
	out, err := b.client.PutObject(ctx, put)
	if err != nil {
		return Object{}, fmt.Errorf("objects: put: %w", err)
	}
	now := time.Now().UTC()
	etag := ""
	if out.ETag != nil {
		etag = strings.Trim(*out.ETag, `"`)
	}
	m := &s3Meta{
		Bucket:      bucket,
		Key:         key,
		ContentType: contentType,
		SizeBytes:   int64(len(body)),
		ETag:        etag,
		State:       UploadStateReady,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := b.writeMeta(ctx, m); err != nil {
		return Object{}, fmt.Errorf("objects: write meta: %w", err)
	}
	return m.toObject(), nil
}

func (b *S3Backend) Stat(ctx context.Context, bucket, key string) (Object, error) {
	m, err := b.readMeta(ctx, bucket, key)
	if err != nil {
		return Object{}, err
	}
	return m.toObject(), nil
}

func (b *S3Backend) DownloadURL(ctx context.Context, bucket, key string, ttl time.Duration) (string, time.Time, error) {
	m, err := b.readMeta(ctx, bucket, key)
	if err != nil {
		return "", time.Time{}, err
	}
	if m.State != UploadStateReady {
		return "", time.Time{}, fmt.Errorf("%w: object not ready", ErrInvalidState)
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	signed, err := b.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.cfg.Bucket),
		Key:    aws.String(dataKey(bucket, key)),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("objects: presign get: %w", err)
	}
	return signed.URL, time.Now().UTC().Add(ttl), nil
}

func (b *S3Backend) Delete(ctx context.Context, bucket, key string) error {
	for _, k := range []string{dataKey(bucket, key), metaKey(bucket, key)} {
		_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(b.cfg.Bucket),
			Key:    aws.String(k),
		})
		if err != nil && !isS3NotFound(err) {
			return fmt.Errorf("objects: delete %s: %w", k, err)
		}
	}
	return nil
}

func (b *S3Backend) List(ctx context.Context, bucket, prefix, pageToken string, pageSize int32) ([]Object, string, error) {
	size := pageSize
	if size <= 0 || size > 1000 {
		size = 100
	}
	in := &s3.ListObjectsV2Input{
		Bucket:  aws.String(b.cfg.Bucket),
		Prefix:  aws.String(bucket + "/" + prefix),
		MaxKeys: aws.Int32(size * 2),
	}
	if pageToken != "" {
		in.ContinuationToken = aws.String(pageToken)
	}
	out, err := b.client.ListObjectsV2(ctx, in)
	if err != nil {
		return nil, "", fmt.Errorf("objects: list: %w", err)
	}
	results := make([]Object, 0, len(out.Contents))
	for _, c := range out.Contents {
		if c.Key == nil || !strings.HasSuffix(*c.Key, metaSuffix) {
			continue
		}
		logicalKey := strings.TrimSuffix(strings.TrimPrefix(*c.Key, bucket+"/"), metaSuffix)
		m, err := b.readMeta(ctx, bucket, logicalKey)
		if err != nil {
			continue
		}
		results = append(results, m.toObject())
		if int32(len(results)) >= size {
			break
		}
	}
	next := ""
	if out.NextContinuationToken != nil && out.IsTruncated != nil && *out.IsTruncated {
		next = *out.NextContinuationToken
	}
	return results, next, nil
}

// isS3NotFound returns true for the common 404-flavored S3 errors:
// NoSuchKey on GetObject, NotFound on HeadObject (no typed error
// returned, only the smithy generic api error code).
func isS3NotFound(err error) bool {
	if err == nil {
		return false
	}
	var nsk *s3types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	var nb *s3types.NoSuchBucket
	if errors.As(err, &nb) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "NoSuchBucket":
			return true
		}
	}
	return false
}
