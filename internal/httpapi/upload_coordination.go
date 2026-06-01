package httpapi

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/coordination"
)

const (
	uploadCoordinationKeyPrefix      = "proofline:upload-operation:v1"
	uploadCoordinationReleaseTimeout = 5 * time.Second
)

func (a *API) acquireUploadCoordinationLease(w http.ResponseWriter, r *http.Request, incidentID string, upload chunkUpload) (coordination.UploadLease, bool) {
	if a.uploadCoordinator == nil || a.uploadCoordinationLeaseTTL <= 0 {
		return coordination.UploadLease{Acquired: true}, true
	}

	lease, err := a.uploadCoordinator.AcquireUploadLease(r.Context(), uploadCoordinationKey(incidentID, upload), a.uploadCoordinationLeaseTTL)
	if err != nil {
		a.logInternalError("acquire upload coordination", err)
		w.Header().Set("Retry-After", retryAfterSeconds(a.uploadCoordinationLeaseTTL))
		writeError(w, http.StatusServiceUnavailable, "upload_coordination_unavailable", "upload coordination is temporarily unavailable")
		return coordination.UploadLease{}, false
	}
	if !lease.Acquired {
		retryAfter := lease.RetryAfter
		if retryAfter <= 0 {
			retryAfter = a.uploadCoordinationLeaseTTL
		}
		w.Header().Set("Retry-After", retryAfterSeconds(retryAfter))
		writeError(w, http.StatusConflict, "upload_in_progress", "upload for this chunk identity is already in progress")
		return coordination.UploadLease{}, false
	}
	return lease, true
}

func (a *API) releaseUploadCoordinationLease(lease coordination.UploadLease) {
	if a.uploadCoordinator == nil || !lease.Acquired || lease.Key == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), uploadCoordinationReleaseTimeout)
	defer cancel()
	if err := a.uploadCoordinator.ReleaseUploadLease(ctx, lease); err != nil {
		a.logInternalError("release upload coordination", err)
	}
}

func uploadCoordinationKey(incidentID string, upload chunkUpload) string {
	var builder strings.Builder
	appendFingerprintField(&builder, "incident_id", incidentID)
	appendFingerprintField(&builder, "stream_id", upload.streamID)
	appendFingerprintField(&builder, "chunk_index", strconv.Itoa(upload.chunkIndex))
	appendFingerprintField(&builder, "media_type", upload.mediaType)
	sum := sha256.Sum256([]byte(builder.String()))
	return fmt.Sprintf("%s:%x", uploadCoordinationKeyPrefix, sum)
}
