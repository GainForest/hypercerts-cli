package cmd

import "testing"

func TestContributorLabel(t *testing.T) {
	tests := []struct {
		name  string
		entry map[string]any
		want  string
	}{
		{
			name: "strongref_only",
			entry: map[string]any{
				"contributorIdentity": map[string]any{
					"uri": "at://did:plc:abc/org.hypercerts.claim.contributorInformation/3k7",
					"cid": "bafyabc",
				},
			},
			want: "at://did:plc:abc/org.hypercerts.claim.contributorInformation/3k7",
		},
		{
			name: "inline_did_only",
			entry: map[string]any{
				"contributorIdentity": map[string]any{
					"$type":    "org.hypercerts.claim.activity#contributorIdentity",
					"identity": "did:plc:z72i",
				},
			},
			want: "did:plc:z72i",
		},
		{
			name: "strongref_with_weight",
			entry: map[string]any{
				"contributorIdentity": map[string]any{
					"uri": "at://did:plc:abc/org.hypercerts.claim.contributorInformation/3k7",
					"cid": "bafyabc",
				},
				"contributionWeight": "45",
			},
			want: "at://did:plc:abc/org.hypercerts.claim.contributorInformation/3k7 (weight: 45)",
		},
		{
			name: "strongref_with_weight_and_role",
			entry: map[string]any{
				"contributorIdentity": map[string]any{
					"uri": "at://did:plc:abc/org.hypercerts.claim.contributorInformation/3k7",
					"cid": "bafyabc",
				},
				"contributionWeight": "45",
				"contributionDetails": map[string]any{
					"$type": "org.hypercerts.claim.activity#contributorRole",
					"role":  "lead researcher",
				},
			},
			want: "at://did:plc:abc/org.hypercerts.claim.contributorInformation/3k7 (weight: 45) [lead researcher]",
		},
		{
			name: "inline_did_with_role",
			entry: map[string]any{
				"contributorIdentity": map[string]any{
					"$type":    "org.hypercerts.claim.activity#contributorIdentity",
					"identity": "did:plc:z72i",
				},
				"contributionDetails": map[string]any{
					"$type": "org.hypercerts.claim.activity#contributorRole",
					"role":  "reviewer",
				},
			},
			want: "did:plc:z72i [reviewer]",
		},
		{
			name:  "missing_identity",
			entry: map[string]any{},
			want:  "unknown",
		},
		{
			name: "nil_identity",
			entry: map[string]any{
				"contributorIdentity": nil,
			},
			want: "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contributorLabel(tt.entry)
			if got != tt.want {
				t.Errorf("contributorLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
