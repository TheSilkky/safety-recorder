package httpapi

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/open-proofline/server/internal/evidencebundle"
	"github.com/open-proofline/server/internal/incidents"
)

func (a *API) downloadPrivateStreamBundle(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incident_id")
	if _, ok := a.authorizeIncident(w, r, incidentID, actionReadCiphertextBundle, dataClassCiphertext); !ok {
		return
	}
	bundle, ok := a.loadCompletedStreamBundle(w, r, incidentID, r.PathValue("stream_id"))
	if !ok {
		return
	}
	a.serveStreamBundle(r.Context(), w, bundle)
}

func (a *API) downloadIncidentViewerStreamBundle(w http.ResponseWriter, r *http.Request) {
	token, ok := a.loadIncidentToken(w, r)
	if !ok {
		return
	}
	bundle, ok := a.loadCompletedStreamBundle(w, r, token.IncidentID, r.PathValue("stream_id"))
	if !ok {
		return
	}
	a.serveStreamBundle(r.Context(), w, bundle)
}

func (a *API) downloadPrivateIncidentBundle(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incident_id")
	if _, ok := a.authorizeIncident(w, r, incidentID, actionReadCiphertextBundle, dataClassCiphertext); !ok {
		return
	}
	detail, err := a.repo.GetIncidentDetail(r.Context(), incidentID)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "incident_not_found", "incident was not found")
		return
	}
	if err != nil {
		a.internalError(w, "get incident detail", err)
		return
	}
	bundles, ok := a.loadCompletedIncidentBundles(w, r, detail.Incident.ID)
	if !ok {
		return
	}
	a.serveIncidentBundle(r.Context(), w, detail, bundles)
}

func (a *API) downloadIncidentViewerIncidentBundle(w http.ResponseWriter, r *http.Request) {
	token, ok := a.loadIncidentToken(w, r)
	if !ok {
		return
	}
	detail, err := a.repo.GetIncidentDetail(r.Context(), token.IncidentID)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "incident_token_invalid", "incident token is invalid, expired, or revoked")
		return
	}
	if err != nil {
		a.internalError(w, "get incident detail", err)
		return
	}
	bundles, ok := a.loadCompletedIncidentBundles(w, r, token.IncidentID)
	if !ok {
		return
	}
	a.serveIncidentBundle(r.Context(), w, detail, bundles)
}

func (a *API) loadCompletedStreamBundle(w http.ResponseWriter, r *http.Request, incidentID, streamID string) (evidencebundle.StreamData, bool) {
	bundle, err := a.buildCompletedStreamBundle(r.Context(), incidentID, streamID)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "stream_not_found", "media stream was not found")
		return evidencebundle.StreamData{}, false
	}
	if errors.Is(err, incidents.ErrInvalidState) {
		writeError(w, http.StatusConflict, "stream_not_complete", "media stream is not complete")
		return evidencebundle.StreamData{}, false
	}
	if isBundleChunkVerificationFailure(err) {
		writeError(w, http.StatusConflict, "stream_bundle_inconsistent", "completed media stream could not be included in stream bundle")
		return evidencebundle.StreamData{}, false
	}
	if err != nil {
		a.internalError(w, "build stream bundle", err)
		return evidencebundle.StreamData{}, false
	}
	return bundle, true
}

func (a *API) loadCompletedIncidentBundles(w http.ResponseWriter, r *http.Request, incidentID string) ([]evidencebundle.StreamData, bool) {
	streams, err := a.repo.ListCompletedMediaStreams(r.Context(), incidentID)
	if err != nil {
		a.internalError(w, "list completed streams", err)
		return nil, false
	}

	bundles := make([]evidencebundle.StreamData, 0, len(streams))
	for _, stream := range streams {
		bundle, err := a.buildCompletedStreamBundle(r.Context(), incidentID, stream.ID)
		if isIncidentBundleInconsistency(err) {
			writeError(w, http.StatusConflict, "incident_bundle_inconsistent", "completed media stream could not be included in incident bundle")
			return nil, false
		}
		if err != nil {
			a.internalError(w, "build stream bundle", err)
			return nil, false
		}
		bundles = append(bundles, bundle)
	}
	return bundles, true
}

func isIncidentBundleInconsistency(err error) bool {
	return errors.Is(err, incidents.ErrInvalidState) ||
		errors.Is(err, incidents.ErrNotFound) ||
		isBundleChunkVerificationFailure(err)
}

func isBundleChunkVerificationFailure(err error) bool {
	return evidencebundle.IsChunkVerificationFailure(err)
}

func (a *API) buildCompletedStreamBundle(ctx context.Context, incidentID, streamID string) (evidencebundle.StreamData, error) {
	stream, err := a.repo.GetMediaStream(ctx, incidentID, streamID)
	if err != nil {
		return evidencebundle.StreamData{}, err
	}
	if stream.Status != incidents.StreamStatusComplete {
		return evidencebundle.StreamData{}, incidents.ErrInvalidState
	}

	chunks, err := a.repo.ListStreamChunks(ctx, incidentID, streamID)
	if err != nil {
		return evidencebundle.StreamData{}, err
	}
	if len(chunks) == 0 {
		return evidencebundle.StreamData{}, incidents.ErrInvalidState
	}
	if !evidencebundle.ValidStreamChunks(stream, chunks) {
		return evidencebundle.StreamData{}, incidents.ErrInvalidState
	}
	if err := evidencebundle.VerifyChunkFiles(ctx, a.store.Open, chunks); err != nil {
		return evidencebundle.StreamData{}, err
	}

	return evidencebundle.NewStreamData(stream, chunks), nil
}

func (a *API) serveStreamBundle(ctx context.Context, w http.ResponseWriter, bundle evidencebundle.StreamData) {
	filename := safeDownloadFilename(fmt.Sprintf("incident_%s_%s_%s.zip", bundle.Stream.IncidentID, bundle.Stream.MediaType, bundle.Stream.ID))
	setBundleHeaders(w, filename)
	if err := writeStreamBundle(w, a.openBundleChunk(ctx), bundle, ""); err != nil {
		a.logInternalError("write stream bundle", err)
	}
}

func (a *API) serveIncidentBundle(ctx context.Context, w http.ResponseWriter, detail incidents.IncidentDetail, bundles []evidencebundle.StreamData) {
	filename := safeDownloadFilename(fmt.Sprintf("incident_%s_evidence.zip", detail.Incident.ID))
	setBundleHeaders(w, filename)

	manifest := evidencebundle.MakeIncidentManifest(detail, bundles)
	zipWriter := zip.NewWriter(w)
	if err := writeJSONZipEntry(zipWriter, "manifest.json", manifest, manifest.GeneratedAt); err != nil {
		a.logInternalError("write incident manifest", err)
		_ = zipWriter.Close()
		return
	}
	for _, bundle := range bundles {
		prefix := "streams/" + safeZipSegment(bundle.Stream.ID) + "/"
		if err := writeStreamBundleToZip(zipWriter, a.openBundleChunk(ctx), bundle, prefix); err != nil {
			a.logInternalError("write incident stream bundle", err)
			_ = zipWriter.Close()
			return
		}
	}
	if err := zipWriter.Close(); err != nil {
		a.logInternalError("close incident bundle", err)
	}
}
