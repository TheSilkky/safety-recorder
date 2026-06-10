package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithy "github.com/aws/smithy-go"
)

func TestS3StoreBlobStoreSemantics(t *testing.T) {
	client := newFakeS3Client()
	store, err := newS3Store(client, S3Options{
		Bucket:  "proofline-evidence",
		Prefix:  "prod/server",
		TempDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("new s3 store: %v", err)
	}
	var blobStore BlobStore = store
	ctx := context.Background()

	payload := "encrypted audio data"
	upload, err := blobStore.SaveTemp(ctx, strings.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("save temp: %v", err)
	}
	if upload.ByteSize != int64(len(payload)) || upload.SHA256Hex != sha256Hex(payload) {
		t.Fatalf("unexpected staged upload metadata: %+v", upload)
	}

	storedPath, err := blobStore.CommitTemp(ctx, upload, "inc_test", "str_test", "audio", 1)
	if err != nil {
		t.Fatalf("commit temp: %v", err)
	}
	if storedPath != "incidents/inc_test/streams/str_test/audio_000001.enc" {
		t.Fatalf("stored path = %q", storedPath)
	}
	if upload.Path != "" {
		t.Fatalf("expected committed upload cleanup, got temp path %q", upload.Path)
	}

	objectKey := "prod/server/incidents/inc_test/streams/str_test/audio_000001.enc"
	if _, ok := client.objects[objectKey]; !ok {
		t.Fatalf("expected committed s3 object %q", objectKey)
	}
	if client.lastPutIfNoneMatch != "*" {
		t.Fatalf("put IfNoneMatch = %q, want *", client.lastPutIfNoneMatch)
	}

	reader, err := blobStore.Open(ctx, storedPath)
	if err != nil {
		t.Fatalf("open stored blob: %v", err)
	}
	stored, err := io.ReadAll(reader)
	if closeErr := reader.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("read stored blob: %v", err)
	}
	if string(stored) != payload {
		t.Fatalf("stored payload mismatch: %q", stored)
	}

	duplicateUpload, err := blobStore.SaveTemp(ctx, strings.NewReader("replacement"), int64(len("replacement")))
	if err != nil {
		t.Fatalf("save duplicate temp: %v", err)
	}
	t.Cleanup(duplicateUpload.Cleanup)
	_, err = blobStore.CommitTemp(ctx, duplicateUpload, "inc_test", "str_test", "audio", 1)
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate commit error = %v, want ErrAlreadyExists", err)
	}
	if string(client.objects[objectKey]) != payload {
		t.Fatalf("duplicate commit overwrote stored payload: %q", string(client.objects[objectKey]))
	}

	if err := blobStore.Remove(ctx, storedPath); err != nil {
		t.Fatalf("remove stored blob: %v", err)
	}
	_, err = blobStore.Open(ctx, storedPath)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("open removed blob error = %v, want os.ErrNotExist", err)
	}
}

func TestS3StoreRejectsUnsafeStoredPaths(t *testing.T) {
	store, err := newS3Store(newFakeS3Client(), S3Options{
		Bucket:  "proofline-evidence",
		TempDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("new s3 store: %v", err)
	}
	ctx := context.Background()

	for _, storedPath := range []string{
		"",
		"/absolute",
		"../escape",
		"incidents/inc_test/../escape",
		"incidents//inc_test/audio_000001.enc",
		"incidents\\inc_test\\audio_000001.enc",
	} {
		if _, err := store.Open(ctx, storedPath); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("open %q error = %v, want ErrUnsafePath", storedPath, err)
		}
		if err := store.Remove(ctx, storedPath); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("remove %q error = %v, want ErrUnsafePath", storedPath, err)
		}
	}
}

func TestS3StoreTempStagingQuota(t *testing.T) {
	store, err := newS3Store(newFakeS3Client(), S3Options{
		Bucket:                "proofline-evidence",
		TempDir:               t.TempDir(),
		TempStagingQuotaBytes: 5,
	})
	if err != nil {
		t.Fatalf("new s3 store: %v", err)
	}
	ctx := context.Background()

	first, err := store.SaveTemp(ctx, strings.NewReader("1234"), 100)
	if err != nil {
		t.Fatalf("save first temp: %v", err)
	}
	t.Cleanup(first.Cleanup)
	if used, err := tempStagingUsedBytes(store.tempDir); err != nil || used != 4 {
		t.Fatalf("temp staging used bytes = %d, err %v; want 4", used, err)
	}

	_, err = store.SaveTemp(ctx, strings.NewReader("12"), 100)
	if !errors.Is(err, ErrTempStagingQuotaExceeded) {
		t.Fatalf("over quota save error = %v, want ErrTempStagingQuotaExceeded", err)
	}
	if used, err := tempStagingUsedBytes(store.tempDir); err != nil || used != 4 {
		t.Fatalf("temp staging used bytes after reject = %d, err %v; want 4", used, err)
	}

	first.Cleanup()
	next, err := store.SaveTemp(ctx, strings.NewReader("12"), 100)
	if err != nil {
		t.Fatalf("save after cleanup: %v", err)
	}
	next.Cleanup()
}

func TestS3StoreRejectsUnsafeCommitSegments(t *testing.T) {
	store, err := newS3Store(newFakeS3Client(), S3Options{
		Bucket:  "proofline-evidence",
		TempDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("new s3 store: %v", err)
	}
	upload, err := store.SaveTemp(context.Background(), strings.NewReader("payload"), int64(len("payload")))
	if err != nil {
		t.Fatalf("save temp: %v", err)
	}
	t.Cleanup(upload.Cleanup)

	_, err = store.CommitTemp(context.Background(), upload, "../escape", "str_test", "audio", 1)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("commit unsafe incident error = %v, want ErrUnsafePath", err)
	}
}

func TestS3StoreRejectsUnsafePrefix(t *testing.T) {
	for _, prefix := range []string{"/absolute", "../escape", "prod/../escape", "prod//server", "prod\\server"} {
		t.Run(prefix, func(t *testing.T) {
			_, err := newS3Store(newFakeS3Client(), S3Options{
				Bucket:  "proofline-evidence",
				Prefix:  prefix,
				TempDir: t.TempDir(),
			})
			if !errors.Is(err, ErrUnsafePath) {
				t.Fatalf("new s3 store error = %v, want ErrUnsafePath", err)
			}
		})
	}
}

func TestS3StoreOpenMissingObjectReturnsNotExist(t *testing.T) {
	store, err := newS3Store(newFakeS3Client(), S3Options{
		Bucket:  "proofline-evidence",
		TempDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("new s3 store: %v", err)
	}

	_, err = store.Open(context.Background(), "incidents/inc_test/streams/str_test/audio_000001.enc")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("open missing object error = %v, want os.ErrNotExist", err)
	}
}

func TestS3StoreUsesCallerContext(t *testing.T) {
	store, err := newS3Store(newFakeS3Client(), S3Options{
		Bucket:  "proofline-evidence",
		TempDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("new s3 store: %v", err)
	}
	upload, err := store.SaveTemp(context.Background(), strings.NewReader("payload"), int64(len("payload")))
	if err != nil {
		t.Fatalf("save temp: %v", err)
	}
	t.Cleanup(upload.Cleanup)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = store.CommitTemp(ctx, upload, "inc_test", "str_test", "audio", 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("commit canceled context error = %v, want context.Canceled", err)
	}
	_, err = store.Open(ctx, "incidents/inc_test/streams/str_test/audio_000001.enc")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("open canceled context error = %v, want context.Canceled", err)
	}
	if err := store.Remove(ctx, "incidents/inc_test/streams/str_test/audio_000001.enc"); !errors.Is(err, context.Canceled) {
		t.Fatalf("remove canceled context error = %v, want context.Canceled", err)
	}
}

func TestS3StoreCheck(t *testing.T) {
	client := newFakeS3Client()
	store, err := newS3Store(client, S3Options{
		Bucket:  "proofline-evidence",
		TempDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("new s3 store: %v", err)
	}

	if err := store.Check(context.Background()); err != nil {
		t.Fatalf("check s3 store: %v", err)
	}
	if client.headBuckets != 1 {
		t.Fatalf("head bucket calls = %d, want 1", client.headBuckets)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Check(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled check error = %v, want context.Canceled", err)
	}
}

type fakeS3Client struct {
	objects            map[string][]byte
	lastPutIfNoneMatch string
	headBuckets        int
}

func newFakeS3Client() *fakeS3Client {
	return &fakeS3Client{objects: make(map[string][]byte)}
}

func (c *fakeS3Client) HeadBucket(ctx context.Context, _ *s3.HeadBucketInput, _ ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.headBuckets++
	return &s3.HeadBucketOutput{}, nil
}

func (c *fakeS3Client) PutObject(ctx context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := aws.ToString(input.Key)
	c.lastPutIfNoneMatch = aws.ToString(input.IfNoneMatch)
	if c.lastPutIfNoneMatch == "*" {
		if _, exists := c.objects[key]; exists {
			return nil, &smithy.GenericAPIError{Code: "PreconditionFailed", Message: "object already exists"}
		}
	}
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	c.objects[key] = body
	return &s3.PutObjectOutput{}, nil
}

func (c *fakeS3Client) GetObject(ctx context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := aws.ToString(input.Key)
	body, ok := c.objects[key]
	if !ok {
		return nil, &smithy.GenericAPIError{Code: "NoSuchKey", Message: "object does not exist"}
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(body))}, nil
}

func (c *fakeS3Client) DeleteObject(ctx context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	delete(c.objects, aws.ToString(input.Key))
	return &s3.DeleteObjectOutput{}, nil
}
