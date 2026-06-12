package httpapi

import (
	"bytes"
	"errors"
	"log/slog"
	"testing"
)

func TestLogInternalErrorDoesNotExposeRawErrorDetail(t *testing.T) {
	var logs bytes.Buffer
	api := &API{logger: slog.New(slog.NewTextHandler(&logs, nil))}
	err := errors.New("backend returned <private endpoint> with <request body> and <wrapped key ciphertext>")

	api.logInternalError("render incident viewer page", err, "route_class", "public_viewer")

	output := logs.String()
	for _, disallowed := range []string{
		"<private endpoint>",
		"<request body>",
		"<wrapped key ciphertext>",
		"err=",
	} {
		if bytes.Contains(logs.Bytes(), []byte(disallowed)) {
			t.Fatalf("internal error log exposed %q: %s", disallowed, output)
		}
	}
	for _, want := range []string{
		"component=httpapi",
		"operation=\"render incident viewer page\"",
		"route_class=public_viewer",
		"error_category=unknown",
	} {
		if !bytes.Contains(logs.Bytes(), []byte(want)) {
			t.Fatalf("internal error log omitted %q: %s", want, output)
		}
	}
}
