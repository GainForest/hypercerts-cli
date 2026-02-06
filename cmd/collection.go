package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/urfave/cli/v3"

	"github.com/GainForest/hypercerts-cli/internal/atproto"
	"github.com/GainForest/hypercerts-cli/internal/menu"
	"github.com/GainForest/hypercerts-cli/internal/prompt"
)

type collectionOption struct {
	URI       string
	CID       string
	Rkey      string
	Title     string
	Type      string
	ItemCount int
	Created   string
}

func fetchCollections(ctx context.Context, client *atclient.APIClient, did string) ([]collectionOption, error) {
	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionCollection)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}
	var result []collectionOption
	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}

		itemCount := 0
		if items := mapSlice(e.Value, "items"); items != nil {
			itemCount = len(items)
		}

		created := ""
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		title := mapStr(e.Value, "title")
		if len(title) > 40 {
			title = title[:37] + "..."
		}

		result = append(result, collectionOption{
			URI:       e.URI,
			CID:       e.CID,
			Rkey:      string(aturi.RecordKey()),
			Title:     title,
			Type:      mapStr(e.Value, "type"),
			ItemCount: itemCount,
			Created:   created,
		})
	}
	return result, nil
}

// promptCollectionItems allows selecting activities and optional weights.
func promptCollectionItems(ctx context.Context, client *atclient.APIClient, w interface{ Write([]byte) (int, error) }) ([]map[string]any, error) {
	did := client.AccountDID.String()
	activities, err := fetchActivities(ctx, client, did)
	if err != nil {
		return nil, err
	}

	if len(activities) == 0 {
		return nil, fmt.Errorf("no activities found - create an activity first")
	}

	selected, err := menu.MultiSelect(w, activities, "activity",
		func(a activityOption) string { return a.Title },
		func(a activityOption) string { return a.Rkey },
	)
	if err != nil {
		return nil, err
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("at least one item is required")
	}

	// Fetch CIDs and optionally prompt for weights
	var items []map[string]any
	for _, a := range selected {
		aturi, _ := syntax.ParseATURI(a.URI)
		_, cid, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
		if err != nil {
			continue
		}

		item := map[string]any{
			"itemIdentifier": buildStrongRef(a.URI, cid),
		}

		// Optionally prompt for weight
		fmt.Fprintf(w, "\n")
		weight, err := prompt.ReadOptionalField(w, os.Stdin, fmt.Sprintf("Weight for '%s'", a.Title), "e.g. 0.5 or 1.0")
		if err != nil {
			return nil, err
		}
		if weight != "" {
			item["itemWeight"] = weight
		}

		items = append(items, item)
	}

	return items, nil
}

func runCollectionCreate(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer

	record := map[string]any{
		"$type":     atproto.CollectionCollection,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	// Title (required)
	title := cmd.String("title")
	if title == "" {
		title, err = prompt.ReadLineWithDefault(w, os.Stdin, "Title", "required, max 80 graphemes", "")
		if err != nil {
			return err
		}
		if title == "" {
			return fmt.Errorf("title is required")
		}
	}
	record["title"] = title

	// Type (optional)
	collType := cmd.String("type")
	if collType == "" {
		collType, err = prompt.ReadOptionalField(w, os.Stdin, "Type", "e.g. project, favorites")
		if err != nil {
			return err
		}
	}
	if collType != "" {
		record["type"] = collType
	}

	// Select activities to include (required)
	fmt.Fprintln(w, "\nSelect activities to include in this collection:")
	items, err := promptCollectionItems(ctx, client, w)
	if err != nil {
		return err
	}
	record["items"] = items

	// Optional fields
	fmt.Fprintln(w)
	if menu.Confirm(w, os.Stdin, "Add optional fields (description, location)?") {
		// Short description
		desc, err := prompt.ReadOptionalField(w, os.Stdin, "Short description", "max 300 graphemes")
		if err != nil {
			return err
		}
		if desc != "" {
			record["shortDescription"] = desc
		}

		// Location
		fmt.Fprintln(w)
		if menu.Confirm(w, os.Stdin, "Add location?") {
			loc, err := selectLocation(ctx, client, w)
			if err != nil {
				return err
			}
			record["location"] = buildStrongRef(loc.URI, loc.CID)
		}
	}

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionCollection, record)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	fmt.Fprintf(w, "\n\033[32m✓\033[0m Created collection: %s\n", uri)
	return nil
}

func runCollectionEdit(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	arg := cmd.Args().First()
	var uri string
	if arg == "" {
		collections, err := fetchCollections(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.SingleSelect(w, collections, "collection",
			func(c collectionOption) string { return c.Title },
			func(c collectionOption) string {
				if c.Type != "" {
					return c.Type
				}
				return fmt.Sprintf("%d items", c.ItemCount)
			},
		)
		if err != nil {
			return err
		}
		uri = selected.URI
	} else {
		uri = resolveRecordURI(did, atproto.CollectionCollection, arg)
	}

	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	existing, cid, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("collection not found: %s", extractRkey(uri))
	}

	// Get current values
	currentTitle := mapStr(existing, "title")
	currentType := mapStr(existing, "type")
	currentDesc := mapStr(existing, "shortDescription")

	changed := false
	isInteractive := cmd.String("title") == "" && cmd.String("type") == ""

	if isInteractive {
		// Title
		newTitle, err := prompt.ReadLineWithDefault(w, os.Stdin, "Title", "required", currentTitle)
		if err != nil {
			return err
		}
		if newTitle != currentTitle {
			existing["title"] = newTitle
			changed = true
		}

		// Type
		newType, err := prompt.ReadLineWithDefault(w, os.Stdin, "Type", "optional", currentType)
		if err != nil {
			return err
		}
		if newType != currentType {
			if newType == "" {
				delete(existing, "type")
			} else {
				existing["type"] = newType
			}
			changed = true
		}

		// Short description
		newDesc, err := prompt.ReadLineWithDefault(w, os.Stdin, "Short description", "optional", currentDesc)
		if err != nil {
			return err
		}
		if newDesc != currentDesc {
			if newDesc == "" {
				delete(existing, "shortDescription")
			} else {
				existing["shortDescription"] = newDesc
			}
			changed = true
		}

		// Location
		existingLoc := mapMap(existing, "location")
		locLabel := "Add location?"
		if existingLoc != nil {
			locLabel = "Replace location?"
		}
		fmt.Fprintln(w)
		if menu.Confirm(w, os.Stdin, locLabel) {
			loc, err := selectLocation(ctx, client, w)
			if err != nil {
				return err
			}
			existing["location"] = buildStrongRef(loc.URI, loc.CID)
			changed = true
		}

		// Items management
		existingItems := mapSlice(existing, "items")
		fmt.Fprintln(w)
		if menu.Confirm(w, os.Stdin, fmt.Sprintf("Manage items? (currently %d)", len(existingItems))) {
			fmt.Fprintln(w, "\nSelect activities to include (replaces current items):")
			items, err := promptCollectionItems(ctx, client, w)
			if err != nil {
				return err
			}
			existing["items"] = items
			changed = true
		}
	} else {
		// Non-interactive mode
		newTitle := cmd.String("title")
		if newTitle != "" && newTitle != currentTitle {
			existing["title"] = newTitle
			changed = true
		}

		newType := cmd.String("type")
		if newType != "" && newType != currentType {
			existing["type"] = newType
			changed = true
		}
	}

	if !changed {
		fmt.Fprintln(w, "No changes.")
		return nil
	}

	resultURI, err := atproto.PutRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String(), existing, &cid)
	if err != nil {
		return fmt.Errorf("failed to update collection: %w", err)
	}

	fmt.Fprintf(w, "\033[32m✓\033[0m Updated collection: %s\n", resultURI)
	return nil
}

func runCollectionDelete(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	id := cmd.String("id")
	if id == "" {
		id = cmd.Args().First()
	}

	if id == "" {
		collections, err := fetchCollections(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.MultiSelect(w, collections, "collection",
			func(c collectionOption) string { return c.Title },
			func(c collectionOption) string {
				if c.Type != "" {
					return c.Type
				}
				return fmt.Sprintf("%d items", c.ItemCount)
			},
		)
		if err != nil {
			return err
		}
		if !menu.ConfirmBulkDelete(w, os.Stdin, len(selected), "collection") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		for _, c := range selected {
			aturi, _ := syntax.ParseATURI(c.URI)
			if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
				fmt.Fprintf(w, "  Warning: %v\n", err)
			} else {
				fmt.Fprintf(w, "Deleted collection: %s\n", c.Rkey)
			}
		}
		return nil
	}

	uri := resolveRecordURI(did, atproto.CollectionCollection, id)
	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, fmt.Sprintf("Delete collection %s?", extractRkey(uri))) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}
	fmt.Fprintf(w, "Deleted collection: %s\n", extractRkey(uri))
	return nil
}

func runCollectionList(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	did := client.AccountDID.String()

	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionCollection)
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	if cmd.Bool("json") {
		var records []map[string]any
		for _, e := range entries {
			records = append(records, map[string]any{"uri": e.URI, "record": e.Value})
		}
		fmt.Fprintln(w, prettyJSON(records))
		return nil
	}

	fmt.Fprintf(w, "\033[1m%-15s %-40s %-15s %-8s %s\033[0m\n", "ID", "TITLE", "TYPE", "ITEMS", "CREATED")
	fmt.Fprintf(w, "%-15s %-40s %-15s %-8s %s\n",
		strings.Repeat("-", 13), strings.Repeat("-", 38),
		strings.Repeat("-", 13), strings.Repeat("-", 6), strings.Repeat("-", 10))

	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		id := string(aturi.RecordKey())

		title := mapStr(e.Value, "title")
		if len(title) > 38 {
			title = title[:35] + "..."
		}

		collType := mapStr(e.Value, "type")
		if collType == "" {
			collType = "-"
		}
		if len(collType) > 13 {
			collType = collType[:10] + "..."
		}

		itemCount := 0
		if items := mapSlice(e.Value, "items"); items != nil {
			itemCount = len(items)
		}

		created := "-"
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		fmt.Fprintf(w, "%-15s %-40s %-15s %-8d %s\n", id, title, collType, itemCount, created)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "\033[90m(no collections found)\033[0m")
	}
	return nil
}

func runCollectionGet(ctx context.Context, cmd *cli.Command) error {
	return runSimpleGet(ctx, cmd, atproto.CollectionCollection, "collection")
}
