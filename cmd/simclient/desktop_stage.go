package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	desktopStageVersion = 1
	stageManifestName   = "manifest.json"
	stageChunksDir      = "chunks"
	stageScratchDir     = "scratch"
)

type desktopStage struct {
	dir        string
	chunksDir  string
	scratchDir string
}

type desktopStageManifest struct {
	Version    int                 `json:"version"`
	IncidentID string              `json:"incident_id"`
	StreamID   string              `json:"stream_id"`
	MediaType  string              `json:"media_type"`
	Source     string              `json:"source"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
	Chunks     []desktopStageChunk `json:"chunks"`
}

type desktopStageChunk struct {
	ChunkIndex     int        `json:"chunk_index"`
	MediaType      string     `json:"media_type"`
	StartedAt      time.Time  `json:"started_at"`
	EndedAt        time.Time  `json:"ended_at"`
	Filename       string     `json:"filename"`
	ByteSize       int64      `json:"byte_size"`
	SHA256Hex      string     `json:"sha256_hex"`
	Uploaded       bool       `json:"uploaded"`
	UploadAttempts int        `json:"upload_attempts"`
	UploadedAt     *time.Time `json:"uploaded_at,omitempty"`
}

func openDesktopStage(dir string) (*desktopStage, error) {
	stage := &desktopStage{
		dir:        dir,
		chunksDir:  filepath.Join(dir, stageChunksDir),
		scratchDir: filepath.Join(dir, stageScratchDir),
	}
	for _, path := range []string{stage.dir, stage.chunksDir, stage.scratchDir} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return nil, safePathError("create desktop staging directory", err)
		}
	}
	return stage, nil
}

func (s *desktopStage) manifestExists() (bool, error) {
	_, err := os.Stat(filepath.Join(s.dir, stageManifestName))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, safePathError("check desktop staging manifest", err)
}

func (s *desktopStage) loadManifest() (desktopStageManifest, error) {
	body, err := os.ReadFile(filepath.Join(s.dir, stageManifestName))
	if err != nil {
		return desktopStageManifest{}, safePathError("read desktop staging manifest", err)
	}
	var manifest desktopStageManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return desktopStageManifest{}, fmt.Errorf("decode desktop staging manifest: %w", err)
	}
	if err := validateDesktopManifest(manifest); err != nil {
		return desktopStageManifest{}, err
	}
	return manifest, nil
}

func (s *desktopStage) saveManifest(manifest desktopStageManifest) error {
	manifest.UpdatedAt = time.Now().UTC()
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode desktop staging manifest: %w", err)
	}
	body = append(body, '\n')
	if err := writeFileAtomic(filepath.Join(s.dir, stageManifestName), body, 0o600); err != nil {
		return safePathError("write desktop staging manifest", err)
	}
	return nil
}

func newDesktopManifest(incidentID, streamID, mediaType, source string) desktopStageManifest {
	now := time.Now().UTC()
	return desktopStageManifest{
		Version:    desktopStageVersion,
		IncidentID: incidentID,
		StreamID:   streamID,
		MediaType:  mediaType,
		Source:     source,
		CreatedAt:  now,
		UpdatedAt:  now,
		Chunks:     []desktopStageChunk{},
	}
}

func validateDesktopManifest(manifest desktopStageManifest) error {
	if manifest.Version != desktopStageVersion {
		return fmt.Errorf("unsupported desktop staging manifest version")
	}
	if strings.TrimSpace(manifest.IncidentID) == "" || strings.TrimSpace(manifest.StreamID) == "" {
		return fmt.Errorf("desktop staging manifest is missing incident or stream identity")
	}
	if !validMediaType(manifest.MediaType) {
		return fmt.Errorf("desktop staging manifest has invalid media type")
	}
	seen := make(map[int]struct{}, len(manifest.Chunks))
	for _, chunk := range manifest.Chunks {
		if chunk.ChunkIndex <= 0 {
			return fmt.Errorf("desktop staging manifest has invalid chunk index")
		}
		if chunk.MediaType != manifest.MediaType {
			return fmt.Errorf("desktop staging manifest chunk media type mismatch")
		}
		if chunk.Filename != cleanStageFilename(chunk.Filename) {
			return fmt.Errorf("desktop staging manifest has invalid chunk filename")
		}
		if chunk.ByteSize <= 0 || !validSHA256Hex(chunk.SHA256Hex) {
			return fmt.Errorf("desktop staging manifest has invalid chunk fingerprint")
		}
		if _, ok := seen[chunk.ChunkIndex]; ok {
			return fmt.Errorf("desktop staging manifest has duplicate chunk index")
		}
		seen[chunk.ChunkIndex] = struct{}{}
	}
	for i := 1; i <= len(manifest.Chunks); i++ {
		if _, ok := seen[i]; !ok {
			return fmt.Errorf("desktop staging manifest has non-contiguous chunk indexes")
		}
	}
	return nil
}

func (s *desktopStage) stageChunk(manifest *desktopStageManifest, encryption simulatorEncryption, sourceBytes []byte, startedAt, endedAt time.Time) error {
	chunkIndex := len(manifest.Chunks) + 1
	body, err := encryption.encryptChunk(manifest.IncidentID, manifest.StreamID, manifest.MediaType, chunkIndex, sourceBytes)
	if err != nil {
		return fmt.Errorf("encrypt staged chunk: %w", err)
	}
	filename := fmt.Sprintf("%s_%06d.enc", manifest.MediaType, chunkIndex)
	finalPath := filepath.Join(s.chunksDir, filename)
	if _, err := os.Stat(finalPath); err == nil {
		return fmt.Errorf("staged chunk already exists")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return safePathError("check staged chunk", err)
	}
	if err := writeFileAtomicNoReplace(finalPath, body, 0o600); err != nil {
		return safePathError("write staged chunk", err)
	}
	manifest.Chunks = append(manifest.Chunks, desktopStageChunk{
		ChunkIndex: chunkIndex,
		MediaType:  manifest.MediaType,
		StartedAt:  startedAt.UTC(),
		EndedAt:    endedAt.UTC(),
		Filename:   filename,
		ByteSize:   int64(len(body)),
		SHA256Hex:  sha256Hex(body),
	})
	return s.saveManifest(*manifest)
}

func (s *desktopStage) chunkUpload(manifest desktopStageManifest, chunk desktopStageChunk) (chunkUpload, error) {
	filename := cleanStageFilename(chunk.Filename)
	if filename == "" || filename != chunk.Filename {
		return chunkUpload{}, fmt.Errorf("invalid staged chunk filename")
	}
	body, err := os.ReadFile(filepath.Join(s.chunksDir, filename))
	if err != nil {
		return chunkUpload{}, safePathError("read staged chunk", err)
	}
	if int64(len(body)) != chunk.ByteSize || sha256Hex(body) != chunk.SHA256Hex {
		return chunkUpload{}, fmt.Errorf("staged chunk fingerprint mismatch")
	}
	return chunkUpload{
		incidentID:     manifest.IncidentID,
		streamID:       manifest.StreamID,
		chunkIndex:     chunk.ChunkIndex,
		mediaType:      manifest.MediaType,
		startedAt:      chunk.StartedAt.UTC(),
		endedAt:        chunk.EndedAt.UTC(),
		filename:       filename,
		body:           body,
		sha256Hex:      chunk.SHA256Hex,
		idempotencyKey: desktopIdempotencyKey(manifest.IncidentID, manifest.StreamID, manifest.MediaType, chunk.ChunkIndex),
	}, nil
}

func sortedScratchFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, safePathError("read recorder scratch directory", err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func cleanStageFilename(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || strings.Contains(value, "/") {
		return ""
	}
	return filepath.Base(value)
}

func writeFileAtomic(path string, body []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(tempPath)
	}
	if _, err := file.Write(body); err != nil {
		cleanup()
		return err
	}
	if err := file.Chmod(perm); err != nil {
		cleanup()
		return err
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}

func writeFileAtomicNoReplace(path string, body []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(tempPath)
	}
	if _, err := file.Write(body); err != nil {
		cleanup()
		return err
	}
	if err := file.Chmod(perm); err != nil {
		cleanup()
		return err
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Link(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return os.Remove(tempPath)
}

func safePathError(action string, err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return fmt.Errorf("%s: %v", action, pathErr.Err)
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return fmt.Errorf("%s: %v", action, linkErr.Err)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func desktopIdempotencyKey(incidentID, streamID, mediaType string, chunkIndex int) string {
	var builder strings.Builder
	builder.WriteString("simclient-desktop-")
	builder.WriteString(incidentID)
	builder.WriteByte('-')
	builder.WriteString(streamID)
	builder.WriteByte('-')
	builder.WriteString(mediaType)
	builder.WriteByte('-')
	builder.WriteString(fmt.Sprintf("%06d", chunkIndex))
	return builder.String()
}

func manifestUploadedCount(manifest desktopStageManifest) int {
	uploaded := 0
	for _, chunk := range manifest.Chunks {
		if chunk.Uploaded {
			uploaded++
		}
	}
	return uploaded
}

func manifestAllUploaded(manifest desktopStageManifest) bool {
	return len(manifest.Chunks) > 0 && manifestUploadedCount(manifest) == len(manifest.Chunks)
}
