package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/GainForest/hypercerts-cli/internal/atproto"
	"github.com/GainForest/hypercerts-cli/internal/github"
)

func TestBuildActivityFromGitHub(t *testing.T) {
	tests := []struct {
		name     string
		repo     *github.RepoInfo
		contribs []createdContributor
		checks   func(t *testing.T, record map[string]any)
	}{
		{
			name: "repo with all fields populated",
			repo: &github.RepoInfo{
				Owner:       "GainForest",
				Name:        "hypercerts-cli",
				FullName:    "GainForest/hypercerts-cli",
				Description: "Hypercerts CLI for managing impact claims",
				HTMLURL:     "https://github.com/GainForest/hypercerts-cli",
				CreatedAt:   "2026-02-03T10:00:00Z",
				PushedAt:    "2026-03-09T12:00:00Z",
				Language:    "Go",
				Topics:      []string{"hypercerts", "cli", "impact"},
				License:     "MIT",
				AvatarURL:   "https://avatars.githubusercontent.com/u/12345",
			},
			contribs: []createdContributor{
				{uri: "at://did:plc:abc/org.hypercerts.claim.contributorInformation/1", cid: "bafyabc", contributions: 50},
				{uri: "at://did:plc:def/org.hypercerts.claim.contributorInformation/2", cid: "bafydef", contributions: 30},
			},
			checks: func(t *testing.T, record map[string]any) {
				if record["$type"] != atproto.CollectionActivity {
					t.Errorf("expected $type %s, got %s", atproto.CollectionActivity, record["$type"])
				}
				if record["title"] != "GainForest/hypercerts-cli" {
					t.Errorf("expected title GainForest/hypercerts-cli, got %s", record["title"])
				}
				if record["shortDescription"] != "Hypercerts CLI for managing impact claims" {
					t.Errorf("expected shortDescription, got %s", record["shortDescription"])
				}
				if record["startDate"] != "2026-02-03T10:00:00Z" {
					t.Errorf("expected startDate 2026-02-03T10:00:00Z, got %s", record["startDate"])
				}
				if _, ok := record["endDate"]; !ok {
					t.Error("expected endDate to be set")
				}

				// Check description
				desc, ok := record["description"].(string)
				if !ok {
					t.Fatal("expected description to be a string")
				}
				if !strings.Contains(desc, "GitHub: https://github.com/GainForest/hypercerts-cli") {
					t.Errorf("expected description to contain GitHub URL, got %s", desc)
				}
				if !strings.Contains(desc, "License: MIT") {
					t.Errorf("expected description to contain license, got %s", desc)
				}
				if !strings.Contains(desc, "Language: Go") {
					t.Errorf("expected description to contain language, got %s", desc)
				}

				// Check workScope is NOT set
				if _, ok := record["workScope"]; ok {
					t.Error("expected workScope to be omitted")
				}

				// Check image
				img, ok := record["image"].(map[string]any)
				if !ok {
					t.Fatal("expected image to be a map")
				}
				if img["uri"] != "https://avatars.githubusercontent.com/u/12345" {
					t.Errorf("expected image uri, got %s", img["uri"])
				}

				// Check contributors
				contribs, ok := record["contributors"].([]any)
				if !ok {
					t.Fatal("expected contributors to be an array")
				}
				if len(contribs) != 2 {
					t.Errorf("expected 2 contributors, got %d", len(contribs))
				}

				// Check first contributor
				c1 := contribs[0].(map[string]any)
				if c1["contributionWeight"] != "50" {
					t.Errorf("expected weight 50, got %s", c1["contributionWeight"])
				}
				ref1 := c1["contributorIdentity"].(map[string]any)
				if ref1["uri"] != "at://did:plc:abc/org.hypercerts.claim.contributorInformation/1" {
					t.Errorf("expected contributor uri, got %s", ref1["uri"])
				}
			},
		},
		{
			name: "repo with empty description",
			repo: &github.RepoInfo{
				Owner:       "TestOrg",
				Name:        "test-repo",
				FullName:    "TestOrg/test-repo",
				Description: "",
				CreatedAt:   "2026-01-01T00:00:00Z",
				Language:    "Python",
				Topics:      []string{},
			},
			contribs: []createdContributor{},
			checks: func(t *testing.T, record map[string]any) {
				shortDesc := record["shortDescription"].(string)
				if !strings.Contains(shortDesc, "TestOrg/test-repo") {
					t.Errorf("expected fallback description to contain repo name, got %s", shortDesc)
				}
				if !strings.Contains(shortDesc, "Python") {
					t.Errorf("expected fallback description to contain language, got %s", shortDesc)
				}
			},
		},
		{
			name: "repo with no language",
			repo: &github.RepoInfo{
				Owner:       "TestOrg",
				Name:        "test-repo",
				FullName:    "TestOrg/test-repo",
				Description: "A test repository",
				HTMLURL:     "https://github.com/TestOrg/test-repo",
				CreatedAt:   "2026-01-01T00:00:00Z",
				Language:    "",
				Topics:      []string{},
			},
			contribs: []createdContributor{},
			checks: func(t *testing.T, record map[string]any) {
				if _, ok := record["workScope"]; ok {
					t.Error("expected workScope to be omitted")
				}
				desc, ok := record["description"].(string)
				if !ok {
					t.Fatal("expected description to be a string")
				}
				if !strings.Contains(desc, "GitHub: https://github.com/TestOrg/test-repo") {
					t.Errorf("expected description to contain GitHub URL, got %s", desc)
				}
				if strings.Contains(desc, "Language:") {
					t.Errorf("expected description to not contain Language when empty, got %s", desc)
				}
			},
		},
		{
			name: "repo with long description",
			repo: &github.RepoInfo{
				Owner:       "TestOrg",
				Name:        "test-repo",
				FullName:    "TestOrg/test-repo",
				Description: strings.Repeat("a", 350),
				CreatedAt:   "2026-01-01T00:00:00Z",
			},
			contribs: []createdContributor{},
			checks: func(t *testing.T, record map[string]any) {
				shortDesc := record["shortDescription"].(string)
				if len(shortDesc) > 300 {
					t.Errorf("expected shortDescription to be truncated to 300 chars, got %d", len(shortDesc))
				}
				if !strings.HasSuffix(shortDesc, "...") {
					t.Errorf("expected truncated description to end with ..., got %s", shortDesc)
				}
			},
		},
		{
			name: "repo with no avatar",
			repo: &github.RepoInfo{
				Owner:       "TestOrg",
				Name:        "test-repo",
				FullName:    "TestOrg/test-repo",
				Description: "Test",
				CreatedAt:   "2026-01-01T00:00:00Z",
				AvatarURL:   "",
			},
			contribs: []createdContributor{},
			checks: func(t *testing.T, record map[string]any) {
				if _, ok := record["image"]; ok {
					t.Error("expected image to be omitted when no avatar URL")
				}
			},
		},
		{
			name: "repo with no contributors",
			repo: &github.RepoInfo{
				Owner:       "TestOrg",
				Name:        "test-repo",
				FullName:    "TestOrg/test-repo",
				Description: "Test",
				CreatedAt:   "2026-01-01T00:00:00Z",
			},
			contribs: []createdContributor{},
			checks: func(t *testing.T, record map[string]any) {
				if _, ok := record["contributors"]; ok {
					t.Error("expected contributors to be omitted when empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := buildActivityFromGitHub(tt.repo, tt.contribs)

			// Common checks
			if record["$type"] != atproto.CollectionActivity {
				t.Errorf("expected $type %s, got %s", atproto.CollectionActivity, record["$type"])
			}
			if record["title"] != tt.repo.FullName {
				t.Errorf("expected title %s, got %s", tt.repo.FullName, record["title"])
			}
			if _, ok := record["createdAt"]; !ok {
				t.Error("expected createdAt to be set")
			}
			if _, ok := record["startDate"]; !ok {
				t.Error("expected startDate to be set")
			}
			if _, ok := record["endDate"]; !ok {
				t.Error("expected endDate to be set")
			}

			// Verify endDate is recent (within 1 minute of now)
			endDate := record["endDate"].(string)
			endTime, err := time.Parse(time.RFC3339, endDate)
			if err != nil {
				t.Errorf("failed to parse endDate: %v", err)
			}
			if time.Since(endTime) > time.Minute {
				t.Errorf("endDate is not recent: %s", endDate)
			}

			// Run custom checks
			tt.checks(t, record)
		})
	}
}
