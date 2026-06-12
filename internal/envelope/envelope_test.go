package envelope

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateKeyProducesAES256Key(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	if len(key.Key) != keySize {
		t.Fatalf("key length = %d, want %d", len(key.Key), keySize)
	}
	if key.Version != keyVersion || key.Scheme != SchemeV1 || key.Algorithm != AlgorithmAES256GCM {
		t.Fatalf("unexpected key metadata: %+v", key)
	}
	if key.KeyID == "" {
		t.Fatal("expected key_id")
	}
}

func TestKeyFileSaveLoadRoundTrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	path := filepath.Join(t.TempDir(), "sim.key.json")

	if err := SaveKeyFile(path, key); err != nil {
		t.Fatalf("SaveKeyFile returned error: %v", err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if stat.Mode().Perm() != 0o600 {
		t.Fatalf("key file mode = %v, want 0600", stat.Mode().Perm())
	}

	loaded, err := LoadKeyFile(path)
	if err != nil {
		t.Fatalf("LoadKeyFile returned error: %v", err)
	}
	if loaded.Version != key.Version || loaded.Scheme != key.Scheme || loaded.Algorithm != key.Algorithm || loaded.KeyID != key.KeyID {
		t.Fatalf("loaded metadata mismatch: got %+v want %+v", loaded, key)
	}
	if !bytes.Equal(loaded.Key, key.Key) {
		t.Fatal("loaded key bytes mismatch")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := mustGenerateKey(t)
	ctx := testChunkContext()
	plaintext := []byte("simulated plaintext")

	envelopeBytes, err := EncryptChunk(key, ctx, plaintext)
	if err != nil {
		t.Fatalf("EncryptChunk returned error: %v", err)
	}
	if bytes.Contains(envelopeBytes, plaintext) {
		t.Fatal("envelope contains plaintext bytes")
	}
	header, err := ParseHeader(envelopeBytes)
	if err != nil {
		t.Fatalf("ParseHeader returned error: %v", err)
	}
	expectedAAD, err := BuildAssociatedData(ctx)
	if err != nil {
		t.Fatalf("BuildAssociatedData returned error: %v", err)
	}
	if header.AAD != string(expectedAAD) {
		t.Fatalf("header AAD = %q, want %q", header.AAD, string(expectedAAD))
	}

	got, err := DecryptChunk(key, ctx, envelopeBytes)
	if err != nil {
		t.Fatalf("DecryptChunk returned error: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("plaintext mismatch: got %q want %q", got, plaintext)
	}
}

func TestDecryptFailsWithWrongKey(t *testing.T) {
	key := mustGenerateKey(t)
	wrongKey := mustGenerateKey(t)
	ctx := testChunkContext()
	envelopeBytes := mustEncrypt(t, key, ctx, []byte("plaintext"))

	if _, err := DecryptChunk(wrongKey, ctx, envelopeBytes); err == nil {
		t.Fatal("DecryptChunk succeeded with wrong key")
	}
}

func TestDecryptFailsWithWrongAssociatedData(t *testing.T) {
	key := mustGenerateKey(t)
	ctx := testChunkContext()
	envelopeBytes := mustEncrypt(t, key, ctx, []byte("plaintext"))

	if _, err := DecryptChunk(key, ChunkContext{
		IncidentID: ctx.IncidentID,
		StreamID:   ctx.StreamID,
		MediaType:  ctx.MediaType,
		ChunkIndex: ctx.ChunkIndex + 1,
	}, envelopeBytes); err == nil {
		t.Fatal("DecryptChunk succeeded with wrong associated data")
	}
}

func TestDecryptFailsWhenMetadataChanges(t *testing.T) {
	key := mustGenerateKey(t)
	ctx := testChunkContext()
	envelopeBytes := mustEncrypt(t, key, ctx, []byte("plaintext"))

	tests := []struct {
		name string
		ctx  ChunkContext
	}{
		{
			name: "incident id",
			ctx: ChunkContext{
				IncidentID: "inc_changed",
				StreamID:   ctx.StreamID,
				MediaType:  ctx.MediaType,
				ChunkIndex: ctx.ChunkIndex,
			},
		},
		{
			name: "stream id",
			ctx: ChunkContext{
				IncidentID: ctx.IncidentID,
				StreamID:   "str_changed",
				MediaType:  ctx.MediaType,
				ChunkIndex: ctx.ChunkIndex,
			},
		},
		{
			name: "media type",
			ctx: ChunkContext{
				IncidentID: ctx.IncidentID,
				StreamID:   ctx.StreamID,
				MediaType:  "video",
				ChunkIndex: ctx.ChunkIndex,
			},
		},
		{
			name: "chunk index",
			ctx: ChunkContext{
				IncidentID: ctx.IncidentID,
				StreamID:   ctx.StreamID,
				MediaType:  ctx.MediaType,
				ChunkIndex: 2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecryptChunk(key, tt.ctx, envelopeBytes); err == nil {
				t.Fatal("DecryptChunk succeeded with changed metadata")
			}
		})
	}
}

func TestParseHeaderRejectsMalformedMagic(t *testing.T) {
	key := mustGenerateKey(t)
	envelopeBytes := mustEncrypt(t, key, testChunkContext(), []byte("plaintext"))
	copy(envelopeBytes[:len(magic)], []byte("WRONG!!\n"))

	if _, err := ParseHeader(envelopeBytes); err == nil {
		t.Fatal("ParseHeader succeeded with malformed magic")
	}
}

func TestParseHeaderRejectsLegacySafetyRecorderMagic(t *testing.T) {
	key := mustGenerateKey(t)
	envelopeBytes := mustEncrypt(t, key, testChunkContext(), []byte("plaintext"))
	copy(envelopeBytes[:len(magic)], []byte("SRCENC1\n"))

	if _, err := ParseHeader(envelopeBytes); err == nil {
		t.Fatal("ParseHeader succeeded with legacy safety-recorder magic")
	}
}

func TestParseHeaderRejectsTruncatedEnvelope(t *testing.T) {
	if _, err := ParseHeader([]byte("PLC")); err == nil {
		t.Fatal("ParseHeader succeeded with truncated envelope")
	}
}

func TestParseHeaderRejectsOversizedHeader(t *testing.T) {
	envelopeBytes := append([]byte(magic), 0, 0, 0, 0)
	binary.BigEndian.PutUint32(envelopeBytes[len(magic):], uint32(maxHeaderLength+1))
	envelopeBytes = append(envelopeBytes, 'x')

	if _, err := ParseHeader(envelopeBytes); err == nil {
		t.Fatal("ParseHeader succeeded with oversized header")
	}
}

func TestParseHeaderRejectsInvalidNonceLength(t *testing.T) {
	key := mustGenerateKey(t)
	envelopeBytes := mustEncrypt(t, key, testChunkContext(), []byte("plaintext"))
	envelopeBytes = rewriteHeader(t, envelopeBytes, func(header *Header) {
		header.NonceB64 = base64.RawURLEncoding.EncodeToString([]byte("short"))
	})

	if _, err := ParseHeader(envelopeBytes); err == nil {
		t.Fatal("ParseHeader succeeded with invalid nonce length")
	}
}

func TestParseHeaderRejectsUnknownAlgorithm(t *testing.T) {
	key := mustGenerateKey(t)
	envelopeBytes := mustEncrypt(t, key, testChunkContext(), []byte("plaintext"))
	envelopeBytes = rewriteHeader(t, envelopeBytes, func(header *Header) {
		header.Algorithm = "AES-256-CBC"
	})

	if _, err := ParseHeader(envelopeBytes); err == nil {
		t.Fatal("ParseHeader succeeded with unknown algorithm")
	}
}

func TestParseHeaderRejectsLegacySafetyRecorderScheme(t *testing.T) {
	key := mustGenerateKey(t)
	envelopeBytes := mustEncrypt(t, key, testChunkContext(), []byte("plaintext"))
	envelopeBytes = rewriteHeader(t, envelopeBytes, func(header *Header) {
		header.Scheme = "safety-recorder-chunk-encryption-v1"
	})

	if _, err := ParseHeader(envelopeBytes); err == nil {
		t.Fatal("ParseHeader succeeded with legacy safety-recorder scheme")
	}
}

func TestBuildAssociatedDataRejectsNewlines(t *testing.T) {
	tests := []struct {
		name string
		ctx  ChunkContext
	}{
		{
			name: "incident id",
			ctx:  ChunkContext{IncidentID: "inc\nbad", StreamID: "str_123", MediaType: "audio", ChunkIndex: 1},
		},
		{
			name: "stream id",
			ctx:  ChunkContext{IncidentID: "inc_123", StreamID: "str\nbad", MediaType: "audio", ChunkIndex: 1},
		},
		{
			name: "media type",
			ctx:  ChunkContext{IncidentID: "inc_123", StreamID: "str_123", MediaType: "audio\nbad", ChunkIndex: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := BuildAssociatedData(tt.ctx); err == nil {
				t.Fatal("BuildAssociatedData succeeded with newline")
			}
		})
	}
}

func TestBuildAssociatedDataRejectsNonPositiveChunkIndex(t *testing.T) {
	for _, index := range []int{0, -1} {
		ctx := testChunkContext()
		ctx.ChunkIndex = index
		if _, err := BuildAssociatedData(ctx); err == nil {
			t.Fatalf("BuildAssociatedData succeeded with chunk index %d", index)
		}
	}
}

func mustGenerateKey(t *testing.T) Key {
	t.Helper()

	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	return key
}

func mustEncrypt(t *testing.T, key Key, ctx ChunkContext, plaintext []byte) []byte {
	t.Helper()

	envelopeBytes, err := EncryptChunk(key, ctx, plaintext)
	if err != nil {
		t.Fatalf("EncryptChunk returned error: %v", err)
	}
	return envelopeBytes
}

func testChunkContext() ChunkContext {
	return ChunkContext{
		IncidentID: "inc_abc",
		StreamID:   "str_def",
		MediaType:  "audio",
		ChunkIndex: 1,
	}
}

func rewriteHeader(t *testing.T, envelopeBytes []byte, mutate func(*Header)) []byte {
	t.Helper()

	headerLength := binary.BigEndian.Uint32(envelopeBytes[len(magic) : len(magic)+4])
	headerStart := len(magic) + 4
	headerEnd := headerStart + int(headerLength)
	var header Header
	if err := json.Unmarshal(envelopeBytes[headerStart:headerEnd], &header); err != nil {
		t.Fatalf("decode header: %v", err)
	}
	mutate(&header)
	updated, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	result := make([]byte, 0, len(magic)+4+len(updated)+len(envelopeBytes[headerEnd:]))
	result = append(result, []byte(magic)...)
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, uint32(len(updated)))
	result = append(result, lengthBytes...)
	result = append(result, updated...)
	result = append(result, envelopeBytes[headerEnd:]...)
	return result
}
