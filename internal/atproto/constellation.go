package atproto

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const ConstellationBase = "https://constellation.microcosm.blue"

// BacklinksSummary is the response from /links/all.
type BacklinksSummary struct {
	Links map[string]map[string]BacklinkCounts `json:"links"`
}

// BacklinkCounts is the count of records and distinct DIDs for a specific collection+path.
type BacklinkCounts struct {
	Records      int `json:"records"`
	DistinctDIDs int `json:"distinct_dids"`
}

// BacklinksResponse is the response from /links (paginated).
type BacklinksResponse struct {
	Total          int             `json:"total"`
	Cursor         *string         `json:"cursor"`
	LinkingRecords []LinkingRecord `json:"linking_records"`
}

// LinkingRecord is a single record that links to the target.
type LinkingRecord struct {
	DID        string `json:"did"`
	Collection string `json:"collection"`
	Rkey       string `json:"rkey"`
}

// GetAllBacklinks returns a summary of all records linking to the target (DID or AT-URI).
func GetAllBacklinks(ctx context.Context, target string) (*BacklinksSummary, error) {
	u, _ := url.Parse(ConstellationBase + "/links/all")
	q := u.Query()
	q.Set("target", target)
	u.RawQuery = q.Encode()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("constellation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("constellation returned %d", resp.StatusCode)
	}

	var result BacklinksSummary
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode backlinks: %w", err)
	}
	return &result, nil
}

// GetBacklinks returns a paginated list of records linking to the target for a specific collection and path.
func GetBacklinks(ctx context.Context, target, collection, path string, cursor string, limit int) (*BacklinksResponse, error) {
	u, _ := url.Parse(ConstellationBase + "/links")
	q := u.Query()
	q.Set("target", target)
	q.Set("collection", collection)
	q.Set("path", path)
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	u.RawQuery = q.Encode()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("constellation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("constellation returned %d", resp.StatusCode)
	}

	var result BacklinksResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode backlinks: %w", err)
	}
	return &result, nil
}

// GetAllBacklinkRecords fetches all linking records for a target+collection+path, handling pagination.
func GetAllBacklinkRecords(ctx context.Context, target, collection, path string) ([]LinkingRecord, error) {
	var all []LinkingRecord
	cursor := ""
	for {
		resp, err := GetBacklinks(ctx, target, collection, path, cursor, 100)
		if err != nil {
			return all, err
		}
		all = append(all, resp.LinkingRecords...)
		if resp.Cursor == nil || *resp.Cursor == "" {
			break
		}
		cursor = *resp.Cursor
	}
	return all, nil
}
