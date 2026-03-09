package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RepoInfo holds parsed GitHub repo metadata.
type RepoInfo struct {
	Owner       string   // e.g. "GainForest"
	Name        string   // e.g. "hypercerts-cli"
	FullName    string   // e.g. "GainForest/hypercerts-cli"
	Description string   // repo description
	HTMLURL     string   // e.g. "https://github.com/GainForest/hypercerts-cli"
	CreatedAt   string   // RFC3339 timestamp
	PushedAt    string   // RFC3339 timestamp
	Language    string   // primary language
	Topics      []string // repo topics
	License     string   // SPDX ID (e.g. "MIT"), empty if none
	AvatarURL   string   // owner avatar URL
}

// Contributor holds a GitHub contributor with commit count.
type Contributor struct {
	Login         string // GitHub username
	HTMLURL       string // profile URL
	AvatarURL     string // avatar URL
	Contributions int    // commit count — used directly as proportional weight
}

// ParseRepo parses "owner/repo", "https://github.com/owner/repo", or
// "https://github.com/owner/repo/..." into (owner, repo) strings.
// Returns error if format is unrecognized.
func ParseRepo(input string) (owner, repo string, err error) {
	if input == "" {
		return "", "", fmt.Errorf("empty repo input")
	}

	// Trim whitespace
	input = strings.TrimSpace(input)

	// If it looks like a URL, parse it
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "github.com/") {
		// Normalize to https:// if needed
		if strings.HasPrefix(input, "github.com/") {
			input = "https://" + input
		}

		u, err := url.Parse(input)
		if err != nil {
			return "", "", fmt.Errorf("invalid URL: %w", err)
		}

		// Must be github.com
		if u.Host != "github.com" {
			return "", "", fmt.Errorf("not a github.com URL: %s", u.Host)
		}

		// Path should be /owner/repo or /owner/repo/...
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 {
			return "", "", fmt.Errorf("invalid GitHub URL path: %s", u.Path)
		}

		return parts[0], parts[1], nil
	}

	// Otherwise, expect "owner/repo" format
	parts := strings.Split(input, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo format: expected owner/repo, got %s", input)
	}

	owner = strings.TrimSpace(parts[0])
	repo = strings.TrimSpace(parts[1])

	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("owner and repo cannot be empty")
	}

	return owner, repo, nil
}

// FetchRepo fetches repo metadata from GET https://api.github.com/repos/{owner}/{repo}.
// token is optional (empty string = unauthenticated).
// Returns error on non-200 status (include status code in error message).
func FetchRepo(ctx context.Context, owner, repo, token string) (*RepoInfo, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "hc-cli")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github returned %d", resp.StatusCode)
	}

	// Parse the GitHub API response
	var apiResp struct {
		Name        string   `json:"name"`
		FullName    string   `json:"full_name"`
		Description string   `json:"description"`
		HTMLURL     string   `json:"html_url"`
		CreatedAt   string   `json:"created_at"`
		PushedAt    string   `json:"pushed_at"`
		Language    string   `json:"language"`
		Topics      []string `json:"topics"`
		License     *struct {
			SPDXID string `json:"spdx_id"`
		} `json:"license"`
		Owner struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		} `json:"owner"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode repo: %w", err)
	}

	info := &RepoInfo{
		Owner:       apiResp.Owner.Login,
		Name:        apiResp.Name,
		FullName:    apiResp.FullName,
		Description: apiResp.Description,
		HTMLURL:     apiResp.HTMLURL,
		CreatedAt:   apiResp.CreatedAt,
		PushedAt:    apiResp.PushedAt,
		Language:    apiResp.Language,
		Topics:      apiResp.Topics,
		AvatarURL:   apiResp.Owner.AvatarURL,
	}

	if apiResp.License != nil {
		info.License = apiResp.License.SPDXID
	}

	return info, nil
}

// FetchContributors fetches contributors from GET https://api.github.com/repos/{owner}/{repo}/contributors.
// Paginates (100 per page) until all contributors are fetched.
// token is optional.
func FetchContributors(ctx context.Context, owner, repo, token string) ([]Contributor, error) {
	var all []Contributor
	page := 1

	for {
		apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contributors?per_page=100&page=%d", owner, repo, page)

		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, "GET", apiURL, nil)
		if err != nil {
			cancel()
			return nil, err
		}

		req.Header.Set("User-Agent", "hc-cli")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("github request failed: %w", err)
		}

		if resp.StatusCode != 200 {
			_ = resp.Body.Close()
			cancel()
			return nil, fmt.Errorf("github returned %d", resp.StatusCode)
		}

		var pageContribs []struct {
			Login         string `json:"login"`
			HTMLURL       string `json:"html_url"`
			AvatarURL     string `json:"avatar_url"`
			Contributions int    `json:"contributions"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&pageContribs); err != nil {
			_ = resp.Body.Close()
			cancel()
			return nil, fmt.Errorf("failed to decode contributors: %w", err)
		}
		_ = resp.Body.Close()
		cancel()

		// Convert to our type
		for _, c := range pageContribs {
			all = append(all, Contributor{
				Login:         c.Login,
				HTMLURL:       c.HTMLURL,
				AvatarURL:     c.AvatarURL,
				Contributions: c.Contributions,
			})
		}

		// If we got fewer than 100, we're done
		if len(pageContribs) < 100 {
			break
		}

		// Check Link header for next page
		linkHeader := resp.Header.Get("Link")
		if !hasNextPage(linkHeader) {
			break
		}

		page++
	}

	return all, nil
}

// hasNextPage checks if the Link header contains rel="next"
func hasNextPage(linkHeader string) bool {
	if linkHeader == "" {
		return false
	}

	// Link header format: <url>; rel="next", <url>; rel="last"
	links := strings.Split(linkHeader, ",")
	for _, link := range links {
		if strings.Contains(link, `rel="next"`) {
			return true
		}
	}
	return false
}
