package atproto

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bluesky-social/indigo/api/agnostic"
	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

// RecordEntry represents a single record from a listing.
type RecordEntry struct {
	URI   string
	CID   string
	Value map[string]any
}

// CreateRecord creates a new record in the given collection.
// Always sets Validate: false for custom unpublished lexicons.
func CreateRecord(ctx context.Context, client *atclient.APIClient, collection string, record map[string]any) (uri, cid string, err error) {
	validate := false
	resp, err := agnostic.RepoCreateRecord(ctx, client, &agnostic.RepoCreateRecord_Input{
		Collection: collection,
		Repo:       client.AccountDID.String(),
		Record:     record,
		Validate:   &validate,
	})
	if err != nil {
		return "", "", err
	}
	return resp.Uri, resp.Cid, nil
}

// GetRecord fetches a single record by DID, collection, and record key.
// Returns the record value, CID, and error.
func GetRecord(ctx context.Context, client *atclient.APIClient, did, collection, rkey string) (map[string]any, string, error) {
	resp, err := agnostic.RepoGetRecord(ctx, client, "", collection, did, rkey)
	if err != nil {
		return nil, "", err
	}
	if resp.Value == nil {
		return nil, "", fmt.Errorf("empty record value")
	}
	var m map[string]any
	if err := json.Unmarshal(*resp.Value, &m); err != nil {
		return nil, "", err
	}
	cid := ""
	if resp.Cid != nil {
		cid = *resp.Cid
	}
	return m, cid, nil
}

// PutRecord updates an existing record with optimistic concurrency via swapCID.
func PutRecord(ctx context.Context, client *atclient.APIClient, did, collection, rkey string, record map[string]any, swapCID *string) (string, error) {
	validate := false
	resp, err := agnostic.RepoPutRecord(ctx, client, &agnostic.RepoPutRecord_Input{
		Collection: collection,
		Repo:       did,
		Record:     record,
		Rkey:       rkey,
		Validate:   &validate,
		SwapRecord: swapCID,
	})
	if err != nil {
		return "", err
	}
	return resp.Uri, nil
}

// DeleteRecord deletes a record by DID, collection, and record key.
func DeleteRecord(ctx context.Context, client *atclient.APIClient, did, collection, rkey string) error {
	_, err := comatproto.RepoDeleteRecord(ctx, client, &comatproto.RepoDeleteRecord_Input{
		Collection: collection,
		Repo:       did,
		Rkey:       rkey,
	})
	return err
}

// ListAllRecords fetches all records in a collection with cursor-based pagination.
func ListAllRecords(ctx context.Context, client *atclient.APIClient, did, collection string) ([]RecordEntry, error) {
	var entries []RecordEntry
	cursor := ""
	for {
		resp, err := agnostic.RepoListRecords(ctx, client, collection, cursor, 100, did, false)
		if err != nil {
			return nil, err
		}
		for _, rec := range resp.Records {
			if rec.Value == nil {
				continue
			}
			var m map[string]any
			if err := json.Unmarshal(*rec.Value, &m); err != nil {
				continue
			}
			entries = append(entries, RecordEntry{
				URI:   rec.Uri,
				CID:   rec.Cid,
				Value: m,
			})
		}
		if resp.Cursor != nil && *resp.Cursor != "" {
			cursor = *resp.Cursor
		} else {
			break
		}
	}
	return entries, nil
}

// ExtractRkey extracts the record key (last segment) from an AT-URI.
func ExtractRkey(uri string) string {
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		parts := strings.Split(strings.TrimPrefix(uri, "at://"), "/")
		if len(parts) >= 3 {
			return parts[2]
		}
		return uri
	}
	return string(aturi.RecordKey())
}
