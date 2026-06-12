package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type streamBundleManifest struct {
	IncidentID string                `json:"incident_id"`
	StreamID   string                `json:"stream_id"`
	MediaType  string                `json:"media_type"`
	ChunkCount int                   `json:"chunk_count"`
	Chunks     []bundleChunkManifest `json:"chunks"`
}

type bundleChunkManifest struct {
	ChunkIndex int    `json:"chunk_index"`
	MediaType  string `json:"media_type"`
	SHA256Hex  string `json:"sha256_hex"`
}

func verifyStreamBundleDecryption(bundleBytes []byte, encryption simulatorEncryption, incidentID, streamID, mediaType string) (int, error) {
	entries, err := readBundleEntries(bundleBytes)
	if err != nil {
		return 0, err
	}

	manifestBody, ok := entries["manifest.json"]
	if !ok {
		return 0, fmt.Errorf("bundle manifest.json is missing")
	}
	var manifest streamBundleManifest
	if err := json.Unmarshal(manifestBody, &manifest); err != nil {
		return 0, fmt.Errorf("decode bundle manifest: %w", err)
	}
	if err := validateBundleManifest(manifest, incidentID, streamID, mediaType); err != nil {
		return 0, err
	}

	verified := 0
	for _, chunk := range manifest.Chunks {
		chunkMediaType := chunk.MediaType
		if chunkMediaType == "" {
			chunkMediaType = manifest.MediaType
		}
		entryName := fmt.Sprintf("chunks/%s_%06d.enc", chunkMediaType, chunk.ChunkIndex)
		ciphertext, ok := entries[entryName]
		if !ok {
			return 0, fmt.Errorf("bundle chunk entry %s is missing", entryName)
		}
		if chunk.SHA256Hex != "" && sha256Hex(ciphertext) != chunk.SHA256Hex {
			return 0, fmt.Errorf("bundle chunk entry %s hash mismatch", entryName)
		}
		if _, err := encryption.decryptChunk(manifest.IncidentID, manifest.StreamID, chunkMediaType, chunk.ChunkIndex, ciphertext); err != nil {
			return 0, fmt.Errorf("decrypt bundle chunk %s: %w", entryName, err)
		}
		verified++
	}
	if verified == 0 {
		return 0, fmt.Errorf("bundle has no encrypted chunks to verify")
	}
	if manifest.ChunkCount != 0 && verified != manifest.ChunkCount {
		return 0, fmt.Errorf("verified %d chunks, manifest expected %d", verified, manifest.ChunkCount)
	}
	return verified, nil
}

func readBundleEntries(bundleBytes []byte) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(bundleBytes), int64(len(bundleBytes)))
	if err != nil {
		return nil, fmt.Errorf("open bundle zip: %w", err)
	}
	entries := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		body, err := readBundleEntry(file)
		if err != nil {
			return nil, err
		}
		entries[file.Name] = body
	}
	return entries, nil
}

func readBundleEntry(file *zip.File) ([]byte, error) {
	handle, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open bundle entry %s: %w", file.Name, err)
	}
	body, readErr := io.ReadAll(handle)
	closeErr := handle.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read bundle entry %s: %w", file.Name, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close bundle entry %s: %w", file.Name, closeErr)
	}
	return body, nil
}

func validateBundleManifest(manifest streamBundleManifest, incidentID, streamID, mediaType string) error {
	if manifest.IncidentID == "" {
		return fmt.Errorf("bundle incident_id is missing")
	}
	if manifest.StreamID == "" {
		return fmt.Errorf("bundle stream_id is missing")
	}
	if manifest.MediaType == "" {
		return fmt.Errorf("bundle media_type is missing")
	}
	if incidentID != "" && manifest.IncidentID != incidentID {
		return fmt.Errorf("bundle incident_id %q does not match %q", manifest.IncidentID, incidentID)
	}
	if streamID != "" && manifest.StreamID != streamID {
		return fmt.Errorf("bundle stream_id %q does not match %q", manifest.StreamID, streamID)
	}
	if mediaType != "" && manifest.MediaType != mediaType {
		return fmt.Errorf("bundle media_type %q does not match %q", manifest.MediaType, mediaType)
	}
	return nil
}
