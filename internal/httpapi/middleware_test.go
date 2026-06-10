package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecoveryMiddlewareLogDoesNotExposePanicValue(t *testing.T) {
	var logs bytes.Buffer
	api := &API{logger: slog.New(slog.NewTextHandler(&logs, nil))}
	handler := api.recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("raw-token-like-value /tmp/proofline/private/data")
	}))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("panic response status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	for _, disallowed := range []string{"raw-token-like-value", "/tmp/proofline/private/data"} {
		if bytes.Contains(logs.Bytes(), []byte(disallowed)) {
			t.Fatalf("panic log exposed %q: %s", disallowed, logs.String())
		}
	}
	if !bytes.Contains(logs.Bytes(), []byte("panic_type=string")) {
		t.Fatalf("panic log omitted safe panic type: %s", logs.String())
	}
	if !bytes.Contains(logs.Bytes(), []byte("component=httpapi")) ||
		!bytes.Contains(logs.Bytes(), []byte("operation=panic_recovery")) ||
		!bytes.Contains(logs.Bytes(), []byte("error_category=unknown")) {
		t.Fatalf("panic log omitted safe structured fields: %s", logs.String())
	}
}

func TestSafeLogPathRedactsMainAPIPathsWithoutMuxPattern(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		target     string
		want       string
		disallowed []string
	}{
		{
			name:   "incident chunk upload",
			method: http.MethodPost,
			target: "/v1/incidents/inc_secret/chunks",
			want:   "POST /v1/incidents/{incident_id}/chunks",
			disallowed: []string{
				"inc_secret",
			},
		},
		{
			name:   "stream bundle download",
			method: http.MethodGet,
			target: "/v1/incidents/inc_secret/streams/str_secret/download?viewer_token=query_secret",
			want:   "GET /v1/incidents/{incident_id}/streams/{stream_id}/download",
			disallowed: []string{
				"inc_secret",
				"str_secret",
				"query_secret",
			},
		},
		{
			name:   "incident token revoke",
			method: http.MethodPost,
			target: "/v1/incident-tokens/itk_secret/revoke",
			want:   "POST /v1/incident-tokens/{token_id}/revoke",
			disallowed: []string{
				"itk_secret",
			},
		},
		{
			name:   "incident token metadata",
			method: http.MethodGet,
			target: "/v1/incidents/inc_secret/incident-tokens/itk_secret",
			want:   "GET /v1/incidents/{incident_id}/incident-tokens/{token_id}",
			disallowed: []string{
				"inc_secret",
				"itk_secret",
			},
		},
		{
			name:   "account recipient key replace",
			method: http.MethodPost,
			target: "/v1/account-recipient-keys/recipient_key_secret/replace",
			want:   "POST /v1/account-recipient-keys/{recipient_key_id}/replace",
			disallowed: []string{
				"recipient_key_secret",
			},
		},
		{
			name:   "admin account password",
			method: http.MethodPost,
			target: "/v1/admin/accounts/acct_secret/password",
			want:   "POST /v1/admin/accounts/{account_id}/password",
			disallowed: []string{
				"acct_secret",
			},
		},
		{
			name:   "admin incident deletion",
			method: http.MethodGet,
			target: "/v1/admin/incidents/inc_secret/deletion",
			want:   "GET /v1/admin/incidents/{incident_id}/deletion",
			disallowed: []string{
				"inc_secret",
			},
		},
		{
			name:   "unknown route",
			method: http.MethodPatch,
			target: "/v1/future/raw_token_like_value",
			want:   "PATCH /v1/{route}",
			disallowed: []string{
				"raw_token_like_value",
			},
		},
		{
			name:   "viewer payload",
			method: http.MethodPost,
			target: "/i/raw_viewer_token_secret/viewer-payload",
			want:   "/i/{token}/viewer-payload",
			disallowed: []string{
				"raw_viewer_token_secret",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.target, nil)

			got := safeLogPath(request)

			if got != tt.want {
				t.Fatalf("safeLogPath() = %q, want %q", got, tt.want)
			}
			for _, disallowed := range tt.disallowed {
				if bytes.Contains([]byte(got), []byte(disallowed)) {
					t.Fatalf("safeLogPath exposed %q in %q", disallowed, got)
				}
			}
		})
	}
}
