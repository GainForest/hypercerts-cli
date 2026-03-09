package github

import (
	"testing"
)

func TestParseRepo(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "simple_owner_slash_repo",
			input:     "GainForest/hypercerts-cli",
			wantOwner: "GainForest",
			wantRepo:  "hypercerts-cli",
			wantErr:   false,
		},
		{
			name:      "https_url",
			input:     "https://github.com/GainForest/hypercerts-cli",
			wantOwner: "GainForest",
			wantRepo:  "hypercerts-cli",
			wantErr:   false,
		},
		{
			name:      "http_url",
			input:     "http://github.com/GainForest/hypercerts-cli",
			wantOwner: "GainForest",
			wantRepo:  "hypercerts-cli",
			wantErr:   false,
		},
		{
			name:      "github_com_without_protocol",
			input:     "github.com/GainForest/hypercerts-cli",
			wantOwner: "GainForest",
			wantRepo:  "hypercerts-cli",
			wantErr:   false,
		},
		{
			name:      "url_with_trailing_path",
			input:     "https://github.com/GainForest/hypercerts-cli/tree/main",
			wantOwner: "GainForest",
			wantRepo:  "hypercerts-cli",
			wantErr:   false,
		},
		{
			name:      "url_with_multiple_trailing_segments",
			input:     "https://github.com/GainForest/hypercerts-cli/blob/main/README.md",
			wantOwner: "GainForest",
			wantRepo:  "hypercerts-cli",
			wantErr:   false,
		},
		{
			name:      "with_whitespace",
			input:     "  GainForest/hypercerts-cli  ",
			wantOwner: "GainForest",
			wantRepo:  "hypercerts-cli",
			wantErr:   false,
		},
		{
			name:      "empty_string",
			input:     "",
			wantOwner: "",
			wantRepo:  "",
			wantErr:   true,
		},
		{
			name:      "just_one_segment",
			input:     "hypercerts-cli",
			wantOwner: "",
			wantRepo:  "",
			wantErr:   true,
		},
		{
			name:      "gitlab_url",
			input:     "https://gitlab.com/foo/bar",
			wantOwner: "",
			wantRepo:  "",
			wantErr:   true,
		},
		{
			name:      "github_url_missing_repo",
			input:     "https://github.com/GainForest",
			wantOwner: "",
			wantRepo:  "",
			wantErr:   true,
		},
		{
			name:      "github_url_missing_owner",
			input:     "https://github.com/",
			wantOwner: "",
			wantRepo:  "",
			wantErr:   true,
		},
		{
			name:      "too_many_slashes",
			input:     "foo/bar/baz",
			wantOwner: "",
			wantRepo:  "",
			wantErr:   true,
		},
		{
			name:      "empty_owner",
			input:     "/hypercerts-cli",
			wantOwner: "",
			wantRepo:  "",
			wantErr:   true,
		},
		{
			name:      "empty_repo",
			input:     "GainForest/",
			wantOwner: "",
			wantRepo:  "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := ParseRepo(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseRepo(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseRepo(%q) unexpected error: %v", tt.input, err)
				return
			}

			if owner != tt.wantOwner {
				t.Errorf("ParseRepo(%q) owner = %q, want %q", tt.input, owner, tt.wantOwner)
			}

			if repo != tt.wantRepo {
				t.Errorf("ParseRepo(%q) repo = %q, want %q", tt.input, repo, tt.wantRepo)
			}
		})
	}
}
