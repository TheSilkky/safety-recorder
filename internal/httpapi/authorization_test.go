package httpapi

import "testing"

func TestIncidentAuthorizationScopeAllowed(t *testing.T) {
	tests := []struct {
		name      string
		action    string
		dataClass string
		want      bool
	}{
		{
			name:      "read metadata",
			action:    actionReadIncident,
			dataClass: dataClassIncidentMetadata,
			want:      true,
		},
		{
			name:      "write metadata",
			action:    actionWriteIncident,
			dataClass: dataClassIncidentMetadata,
			want:      true,
		},
		{
			name:      "write ciphertext",
			action:    actionWriteIncident,
			dataClass: dataClassCiphertext,
			want:      true,
		},
		{
			name:      "read ciphertext bundle",
			action:    actionReadCiphertextBundle,
			dataClass: dataClassCiphertext,
			want:      true,
		},
		{
			name:      "create public link",
			action:    actionCreatePublicLink,
			dataClass: dataClassPublicLinkGrant,
			want:      true,
		},
		{
			name:      "read public link",
			action:    actionReadPublicLink,
			dataClass: dataClassPublicLinkGrant,
			want:      true,
		},
		{
			name:      "revoke public link",
			action:    actionRevokePublicLink,
			dataClass: dataClassPublicLinkGrant,
			want:      true,
		},
		{
			name:      "read raw keys",
			action:    actionReadIncident,
			dataClass: "raw_keys",
			want:      false,
		},
		{
			name:      "delete incident",
			action:    "delete_incident",
			dataClass: dataClassIncidentMetadata,
			want:      true,
		},
		{
			name:      "unknown action",
			action:    "delete_everything",
			dataClass: dataClassIncidentMetadata,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := incidentAuthorizationScopeAllowed(tt.action, tt.dataClass); got != tt.want {
				t.Fatalf("incidentAuthorizationScopeAllowed(%q, %q) = %v, want %v", tt.action, tt.dataClass, got, tt.want)
			}
		})
	}
}
