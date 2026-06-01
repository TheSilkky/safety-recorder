package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithy "github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// S3Options configures the optional S3-compatible blob backend.
type S3Options struct {
	Endpoint        string
	Region          string
	Bucket          string
	Prefix          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	ForcePathStyle  bool
	TempDir         string
}

// S3Store stores committed encrypted blobs in an S3-compatible object store.
// Upload bytes are still staged in a local temp directory so hash verification
// happens before any final object write.
type S3Store struct {
	client  s3ObjectClient
	bucket  string
	prefix  string
	tempDir string
}

type s3ObjectClient interface {
	HeadBucket(context.Context, *s3.HeadBucketInput, ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type staticS3Credentials struct {
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
}

var _ BlobStore = (*S3Store)(nil)

// NewS3 creates an S3-compatible encrypted blob store.
func NewS3(opts S3Options) (*S3Store, error) {
	opts.Endpoint = strings.TrimSpace(opts.Endpoint)
	opts.Region = strings.TrimSpace(opts.Region)
	opts.AccessKeyID = strings.TrimSpace(opts.AccessKeyID)
	opts.SecretAccessKey = strings.TrimSpace(opts.SecretAccessKey)
	opts.SessionToken = strings.TrimSpace(opts.SessionToken)
	if opts.Endpoint == "" {
		return nil, fmt.Errorf("missing s3 endpoint")
	}
	if opts.Region == "" {
		return nil, fmt.Errorf("missing s3 region")
	}
	if opts.AccessKeyID == "" && opts.SecretAccessKey != "" {
		return nil, fmt.Errorf("s3 access key id is required when secret access key is set")
	}
	if opts.AccessKeyID != "" && opts.SecretAccessKey == "" {
		return nil, fmt.Errorf("s3 secret access key is required when access key id is set")
	}
	if opts.AccessKeyID == "" {
		return nil, fmt.Errorf("missing s3 access key id")
	}

	awsCfg := aws.Config{
		Region: opts.Region,
		Credentials: aws.NewCredentialsCache(staticS3Credentials{
			accessKeyID:     opts.AccessKeyID,
			secretAccessKey: opts.SecretAccessKey,
			sessionToken:    opts.SessionToken,
		}),
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(opts.Endpoint)
		options.UsePathStyle = opts.ForcePathStyle
	})
	return newS3Store(client, opts)
}

func (p staticS3Credentials) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:     p.accessKeyID,
		SecretAccessKey: p.secretAccessKey,
		SessionToken:    p.sessionToken,
		Source:          "proofline static s3 config",
	}, nil
}

func newS3Store(client s3ObjectClient, opts S3Options) (*S3Store, error) {
	if client == nil {
		return nil, fmt.Errorf("missing s3 client")
	}
	if strings.TrimSpace(opts.Bucket) == "" {
		return nil, fmt.Errorf("missing s3 bucket")
	}
	if strings.TrimSpace(opts.TempDir) == "" {
		return nil, fmt.Errorf("missing s3 temp directory")
	}
	prefix, err := cleanS3Prefix(opts.Prefix)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(opts.TempDir, 0o700); err != nil {
		return nil, fmt.Errorf("create s3 temp directory: %w", err)
	}
	return &S3Store{
		client:  client,
		bucket:  strings.TrimSpace(opts.Bucket),
		prefix:  prefix,
		tempDir: opts.TempDir,
	}, nil
}

// Check verifies that local temp staging is writable and the configured bucket
// is reachable.
func (s *S3Store) Check(ctx context.Context) error {
	if err := checkWritableDir(ctx, s.tempDir); err != nil {
		return fmt.Errorf("check s3 temp storage: %w", err)
	}
	if _, err := s.client.HeadBucket(s3Context(ctx), &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	}); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("check s3 bucket: %w", err)
	}
	return nil
}

// SaveTemp streams reader into a local temporary file, enforcing maxBytes and
// computing SHA-256 before an object is committed to S3.
func (s *S3Store) SaveTemp(_ context.Context, reader io.Reader, maxBytes int64) (*TempUpload, error) {
	return saveTempToDir(s.tempDir, reader, maxBytes)
}

// CommitTemp writes a verified temp upload to a server-controlled immutable S3
// object key. It uses If-None-Match: * so an existing object is never replaced.
func (s *S3Store) CommitTemp(ctx context.Context, upload *TempUpload, incidentID, streamID, mediaType string, chunkIndex int) (string, error) {
	if upload == nil || upload.Path == "" {
		return "", fmt.Errorf("missing temp upload")
	}
	storedPath, err := storedBlobPath(incidentID, streamID, mediaType, chunkIndex)
	if err != nil {
		return "", err
	}
	key, err := s.objectKey(storedPath)
	if err != nil {
		return "", err
	}

	file, err := os.Open(upload.Path)
	if err != nil {
		return "", fmt.Errorf("open temp upload: %w", err)
	}
	defer file.Close()

	_, err = s.client.PutObject(s3Context(ctx), &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          file,
		ContentLength: aws.Int64(upload.ByteSize),
		ContentType:   aws.String("application/octet-stream"),
		IfNoneMatch:   aws.String("*"),
	})
	if isS3AlreadyExists(err) {
		return "", ErrAlreadyExists
	}
	if err != nil {
		return "", fmt.Errorf("put s3 object: %w", err)
	}
	_ = os.Remove(upload.Path)
	upload.Path = ""

	return storedPath, nil
}

// Open opens a previously committed blob by its stored relative path.
func (s *S3Store) Open(ctx context.Context, storedPath string) (io.ReadCloser, error) {
	key, err := s.objectKey(storedPath)
	if err != nil {
		return nil, err
	}
	output, err := s.client.GetObject(s3Context(ctx), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if isS3MissingObject(err) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("get s3 object: %w", err)
	}
	return output.Body, nil
}

// Remove deletes a committed blob by its stored relative path.
func (s *S3Store) Remove(ctx context.Context, storedPath string) error {
	key, err := s.objectKey(storedPath)
	if err != nil {
		return err
	}
	_, err = s.client.DeleteObject(s3Context(ctx), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete s3 object: %w", err)
	}
	return nil
}

func (s *S3Store) objectKey(storedPath string) (string, error) {
	clean, err := cleanStoredPath(storedPath)
	if err != nil {
		return "", err
	}
	if s.prefix == "" {
		return clean, nil
	}
	return path.Join(s.prefix, clean), nil
}

func s3Context(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func cleanS3Prefix(prefix string) (string, error) {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return "", nil
	}
	if path.IsAbs(trimmed) || strings.Contains(trimmed, "\\") {
		return "", ErrUnsafePath
	}
	trimmed = strings.TrimSuffix(trimmed, "/")
	for _, segment := range strings.Split(trimmed, "/") {
		if !safePathSegment(segment) {
			return "", ErrUnsafePath
		}
	}
	return trimmed, nil
}

func isS3AlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "PreconditionFailed", "ConditionalRequestConflict":
			return true
		}
	}
	var responseErr *smithyhttp.ResponseError
	if errors.As(err, &responseErr) {
		status := responseErr.HTTPStatusCode()
		return status == http.StatusPreconditionFailed || status == http.StatusConflict
	}
	return false
}

func isS3MissingObject(err error) bool {
	if err == nil {
		return false
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound":
			return true
		}
	}
	var responseErr *smithyhttp.ResponseError
	if errors.As(err, &responseErr) {
		return responseErr.HTTPStatusCode() == http.StatusNotFound
	}
	return false
}
