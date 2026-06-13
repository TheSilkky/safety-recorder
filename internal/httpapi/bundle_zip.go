package httpapi

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/evidencebundle"
	"github.com/open-proofline/server/internal/incidents"
)

func (a *API) openBundleChunk(ctx context.Context) func(string) (io.ReadCloser, error) {
	return func(relPath string) (io.ReadCloser, error) {
		return a.store.Open(ctx, relPath)
	}
}

func writeStreamBundle(w io.Writer, openChunk func(string) (io.ReadCloser, error), bundle evidencebundle.StreamData, prefix string) error {
	zipWriter := zip.NewWriter(w)
	if err := writeStreamBundleToZip(zipWriter, openChunk, bundle, prefix); err != nil {
		_ = zipWriter.Close()
		return err
	}
	return zipWriter.Close()
}

func writeStreamBundleToZip(zipWriter *zip.Writer, openChunk func(string) (io.ReadCloser, error), bundle evidencebundle.StreamData, prefix string) error {
	if err := writeJSONZipEntry(zipWriter, prefix+"manifest.json", bundle.Manifest, bundle.Stream.UpdatedAt); err != nil {
		return err
	}
	for _, chunk := range bundle.Chunks {
		entryName := fmt.Sprintf("%schunks/%s_%06d.enc", prefix, safeZipSegment(chunk.MediaType), chunk.ChunkIndex)
		if err := writeChunkZipEntry(zipWriter, openChunk, entryName, chunk); err != nil {
			return err
		}
	}
	return nil
}

func writeChunkZipEntry(zipWriter *zip.Writer, openChunk func(string) (io.ReadCloser, error), entryName string, chunk incidents.Chunk) error {
	header := &zip.FileHeader{
		Name:     entryName,
		Method:   zip.Deflate,
		Modified: chunk.CreatedAt,
	}
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create chunk zip entry: %w", err)
	}

	file, err := openChunk(chunk.StoredPath)
	if err != nil {
		return fmt.Errorf("open chunk: %w", err)
	}
	defer file.Close()
	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("copy chunk: %w", err)
	}
	return nil
}

func writeJSONZipEntry(zipWriter *zip.Writer, name string, value any, modified time.Time) error {
	header := &zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: modified,
	}
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create json zip entry: %w", err)
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json zip entry: %w", err)
	}
	encoded = append(encoded, '\n')
	if _, err := writer.Write(encoded); err != nil {
		return fmt.Errorf("write json zip entry: %w", err)
	}
	return nil
}

func setBundleHeaders(w http.ResponseWriter, filename string) {
	setPublicBrowserSecurityHeaders(w)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	setNoStore(w)
}

func safeDownloadFilename(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' || char == '.' {
			builder.WriteRune(char)
			continue
		}
		builder.WriteByte('_')
	}
	filename := builder.String()
	if filename == "" || filename == "." || filename == ".." {
		return "evidence.zip"
	}
	return filename
}

func safeZipSegment(value string) string {
	return strings.Trim(safeDownloadFilename(value), ".")
}
