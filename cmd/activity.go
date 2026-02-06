package cmd

import (
	"context"
	"fmt"
	"io"
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

// optionalField represents an optional field that can be added to an activity.
type optionalField struct {
	Name  string
	Label string
	Hint  string
}

var activityOptionalFields = []optionalField{
	{Name: "description", Label: "Description", Hint: "longer description, max 3000 chars"},
	{Name: "workScope", Label: "Work scope (free-form)", Hint: "scope of the work as text"},
	{Name: "workScopeTag", Label: "Work scope (tag)", Hint: "link to a reusable scope tag"},
	{Name: "startDate", Label: "Start date", Hint: "YYYY-MM-DD or RFC3339"},
	{Name: "endDate", Label: "End date", Hint: "YYYY-MM-DD or RFC3339"},
	{Name: "contributors", Label: "Contributors", Hint: "add contributor references"},
	{Name: "locations", Label: "Locations", Hint: "geographic locations"},
	{Name: "rights", Label: "Rights", Hint: "rights/license definition"},
	{Name: "image", Label: "Image", Hint: "image URI for the hypercert"},
}

type activityOption struct {
	URI     string
	Rkey    string
	Title   string
	Created string
}

func fetchActivities(ctx context.Context, client *atclient.APIClient, did string) ([]activityOption, error) {
	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionActivity)
	if err != nil {
		return nil, fmt.Errorf("failed to list activities: %w", err)
	}
	var result []activityOption
	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		title := mapStr(e.Value, "title")
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		created := ""
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}
		result = append(result, activityOption{
			URI:     e.URI,
			Rkey:    string(aturi.RecordKey()),
			Title:   title,
			Created: created,
		})
	}
	return result, nil
}

func runActivityCreate(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer

	record := map[string]any{
		"$type":     atproto.CollectionActivity,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	// Required: title
	title := cmd.String("title")
	if title == "" {
		title, err = prompt.ReadLineWithDefault(w, os.Stdin, "Title", "required, max 256 chars", "")
		if err != nil {
			return err
		}
		if title == "" {
			return fmt.Errorf("title is required")
		}
	}
	record["title"] = title

	// Required: shortDescription
	shortDesc := cmd.String("description")
	if shortDesc == "" {
		shortDesc, err = prompt.ReadLineWithDefault(w, os.Stdin, "Short description", "required, max 300 chars", "")
		if err != nil {
			return err
		}
		if shortDesc == "" {
			return fmt.Errorf("short description is required")
		}
	}
	record["shortDescription"] = shortDesc

	// Handle optional flags (non-interactive mode)
	hasFlags := false
	if s := cmd.String("start-date"); s != "" {
		record["startDate"] = normalizeDate(s)
		hasFlags = true
	}
	if s := cmd.String("end-date"); s != "" {
		record["endDate"] = normalizeDate(s)
		hasFlags = true
	}
	if s := cmd.String("work-scope"); s != "" {
		record["workScope"] = map[string]any{
			"$type": atproto.CollectionActivity + "#workScopeString",
			"scope": s,
		}
		hasFlags = true
	}

	// Interactive optional fields
	if !hasFlags {
		fmt.Fprintln(w)
		if menu.Confirm(w, os.Stdin, "Add optional fields?") {
			selected, err := menu.MultiSelect(w, activityOptionalFields, "field",
				func(f optionalField) string { return f.Label },
				func(f optionalField) string { return f.Hint },
			)
			if err != nil && err != menu.ErrCancelled {
				return err
			}

			for _, field := range selected {
				if err := promptOptionalField(ctx, client, w, record, field); err != nil {
					return err
				}
			}
		}
	}

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionActivity, record)
	if err != nil {
		return fmt.Errorf("failed to create activity: %w", err)
	}

	fmt.Fprintf(w, "\n\033[32m\u2713\033[0m Created activity: %s\n", uri)
	return nil
}

func promptOptionalField(ctx context.Context, client *atclient.APIClient, w io.Writer, record map[string]any, field optionalField) error {
	switch field.Name {
	case "description":
		val, err := prompt.ReadOptionalField(w, os.Stdin, "Description", "max 3000 chars")
		if err != nil {
			return err
		}
		if val != "" {
			record["description"] = val
		}
	case "workScope":
		val, err := prompt.ReadOptionalField(w, os.Stdin, "Work scope", "free-form scope description")
		if err != nil {
			return err
		}
		if val != "" {
			record["workScope"] = map[string]any{
				"$type": atproto.CollectionActivity + "#workScopeString",
				"scope": val,
			}
		}
	case "startDate":
		val, err := prompt.ReadOptionalField(w, os.Stdin, "Start date", "YYYY-MM-DD or RFC3339")
		if err != nil {
			return err
		}
		if val != "" {
			record["startDate"] = normalizeDate(val)
		}
	case "endDate":
		val, err := prompt.ReadOptionalField(w, os.Stdin, "End date", "YYYY-MM-DD or RFC3339")
		if err != nil {
			return err
		}
		if val != "" {
			record["endDate"] = normalizeDate(val)
		}
	case "contributors":
		contributors, err := promptContributors(ctx, client, w)
		if err != nil {
			return err
		}
		if len(contributors) > 0 {
			record["contributors"] = contributors
		}
	case "locations":
		locations, err := selectLocations(ctx, client, w)
		if err != nil {
			return err
		}
		if len(locations) > 0 {
			var refs []any
			for _, loc := range locations {
				refs = append(refs, buildStrongRef(loc.URI, loc.CID))
			}
			record["locations"] = refs
		}
	case "rights":
		rights, err := selectRights(ctx, client, w)
		if err != nil {
			return err
		}
		record["rights"] = buildStrongRef(rights.URI, rights.CID)
	case "workScopeTag":
		scope, err := selectWorkScope(ctx, client, w)
		if err != nil {
			return err
		}
		record["workScope"] = map[string]any{
			"$type": atproto.CollectionActivity + "#workScopeRef",
			"scope": buildStrongRef(scope.URI, scope.CID),
		}
	case "image":
		val, err := prompt.ReadOptionalField(w, os.Stdin, "Image URI", "URL to hypercert image")
		if err != nil {
			return err
		}
		if val != "" {
			record["image"] = map[string]any{
				"$type": "org.hypercerts.defs#uri",
				"uri":   val,
			}
		}
	}
	return nil
}

func promptContributors(ctx context.Context, client *atclient.APIClient, w io.Writer) ([]any, error) {
	var contributors []any
	for i := 1; ; i++ {
		fmt.Fprintf(w, "\n  \033[1mContributor %d\033[0m\n", i)

		contributor, err := selectContributor(ctx, client, w)
		if err != nil {
			if err == menu.ErrCancelled {
				break
			}
			return nil, err
		}

		contribObj := map[string]any{
			"contributorIdentity": map[string]any{
				"uri": contributor.URI,
				"cid": contributor.CID,
			},
		}

		role, err := prompt.ReadOptionalField(w, os.Stdin, "  Role", "optional")
		if err != nil {
			return nil, err
		}
		if role != "" {
			contribObj["contributionDetails"] = map[string]any{
				"$type": atproto.CollectionActivity + "#contributorRole",
				"role":  role,
			}
		}

		weight, err := prompt.ReadOptionalField(w, os.Stdin, "  Weight", "optional, numeric")
		if err != nil {
			return nil, err
		}
		if weight != "" {
			contribObj["contributionWeight"] = weight
		}

		contributors = append(contributors, contribObj)
		fmt.Fprintf(w, "  \033[32m\u2713\033[0m Added contributor\n")

		if !menu.Confirm(w, os.Stdin, "\nAdd another contributor?") {
			break
		}
	}
	return contributors, nil
}

func runActivityEdit(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	arg := cmd.Args().First()
	var uri string
	if arg == "" {
		claims, err := fetchActivities(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.SingleSelect(w, claims, "activity",
			func(c activityOption) string { return c.Title },
			func(c activityOption) string { return c.Created },
		)
		if err != nil {
			return err
		}
		uri = selected.URI
	} else {
		uri = resolveRecordURI(did, atproto.CollectionActivity, arg)
	}

	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	existing, cid, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("activity not found: %s", extractRkey(uri))
	}

	changed := false

	// Apply flags if provided
	if t := cmd.String("title"); t != "" {
		existing["title"] = t
		changed = true
	}
	if d := cmd.String("description"); d != "" {
		existing["shortDescription"] = d
		changed = true
	}
	if s := cmd.String("start-date"); s != "" {
		existing["startDate"] = normalizeDate(s)
		changed = true
	}
	if s := cmd.String("end-date"); s != "" {
		existing["endDate"] = normalizeDate(s)
		changed = true
	}
	if s := cmd.String("work-scope"); s != "" {
		existing["workScope"] = map[string]any{
			"$type": atproto.CollectionActivity + "#workScopeString",
			"scope": s,
		}
		changed = true
	}

	// Interactive edit if no flags
	if !changed {
		newTitle, err := prompt.ReadLineWithDefault(w, os.Stdin, "Title", "", mapStr(existing, "title"))
		if err != nil {
			return err
		}
		if newTitle != mapStr(existing, "title") {
			existing["title"] = newTitle
			changed = true
		}

		newDesc, err := prompt.ReadLineWithDefault(w, os.Stdin, "Short description", "", mapStr(existing, "shortDescription"))
		if err != nil {
			return err
		}
		if newDesc != mapStr(existing, "shortDescription") {
			existing["shortDescription"] = newDesc
			changed = true
		}

		fmt.Fprintln(w)
		if menu.Confirm(w, os.Stdin, "Edit optional fields?") {
			// Dates
			newStart, err := prompt.ReadLineWithDefault(w, os.Stdin, "Start date", "YYYY-MM-DD", mapStr(existing, "startDate"))
			if err != nil {
				return err
			}
			if newStart != "" && newStart != mapStr(existing, "startDate") {
				existing["startDate"] = normalizeDate(newStart)
				changed = true
			}

			newEnd, err := prompt.ReadLineWithDefault(w, os.Stdin, "End date", "YYYY-MM-DD", mapStr(existing, "endDate"))
			if err != nil {
				return err
			}
			if newEnd != "" && newEnd != mapStr(existing, "endDate") {
				existing["endDate"] = normalizeDate(newEnd)
				changed = true
			}

			// Locations
			existingLocs := mapSlice(existing, "locations")
			locLabel := "Add locations?"
			if len(existingLocs) > 0 {
				locLabel = fmt.Sprintf("Replace %d location(s)?", len(existingLocs))
			}
			fmt.Fprintln(w)
			if menu.Confirm(w, os.Stdin, locLabel) {
				locations, err := selectLocations(ctx, client, w)
				if err != nil {
					return err
				}
				if len(locations) > 0 {
					var refs []any
					for _, loc := range locations {
						refs = append(refs, buildStrongRef(loc.URI, loc.CID))
					}
					existing["locations"] = refs
					changed = true
				}
			}

			// Rights
			existingRights := mapMap(existing, "rights")
			rightsLabel := "Add rights?"
			if existingRights != nil {
				rightsLabel = "Replace rights?"
			}
			fmt.Fprintln(w)
			if menu.Confirm(w, os.Stdin, rightsLabel) {
				rights, err := selectRights(ctx, client, w)
				if err != nil {
					return err
				}
				existing["rights"] = buildStrongRef(rights.URI, rights.CID)
				changed = true
			}

			// Image
			existingImage := mapMap(existing, "image")
			imageLabel := "Add image URI?"
			currentImageURI := ""
			if existingImage != nil {
				currentImageURI = mapStr(existingImage, "uri")
				if currentImageURI != "" {
					imageLabel = "Replace image URI?"
				}
			}
			fmt.Fprintln(w)
			if menu.Confirm(w, os.Stdin, imageLabel) {
				newImage, err := prompt.ReadLineWithDefault(w, os.Stdin, "Image URI", "URL to hypercert image", currentImageURI)
				if err != nil {
					return err
				}
				if newImage != "" && newImage != currentImageURI {
					existing["image"] = map[string]any{
						"$type": "org.hypercerts.defs#uri",
						"uri":   newImage,
					}
					changed = true
				}
			}
		}
	}

	if !changed {
		fmt.Fprintln(w, "No changes.")
		return nil
	}

	resultURI, err := atproto.PutRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String(), existing, &cid)
	if err != nil {
		return fmt.Errorf("failed to update activity: %w", err)
	}

	fmt.Fprintf(w, "\033[32m\u2713\033[0m Updated activity: %s\n", resultURI)
	return nil
}

func runActivityDelete(ctx context.Context, cmd *cli.Command) error {
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
		claims, err := fetchActivities(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.MultiSelect(w, claims, "activity",
			func(c activityOption) string { return c.Rkey },
			func(c activityOption) string {
				info := c.Title
				if c.Created != "" {
					info += "  " + c.Created
				}
				return info
			},
		)
		if err != nil {
			return err
		}
		if !menu.ConfirmBulkDelete(w, os.Stdin, len(selected), "activity") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		for _, claim := range selected {
			if err := deleteActivity(ctx, client, w, did, claim.URI, true); err != nil {
				fmt.Fprintf(w, "  Warning: %v\n", err)
			}
		}
		return nil
	}

	uri := resolveRecordURI(did, atproto.CollectionActivity, id)
	return deleteActivity(ctx, client, w, did, uri, cmd.Bool("force"))
}

func deleteActivity(ctx context.Context, client *atclient.APIClient, w io.Writer, did, uri string, force bool) error {
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	// Verify exists
	_, _, err = atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("activity not found: %s", extractRkey(uri))
	}

	// Find linked records
	measurementURIs := findLinkedURIs(ctx, client, did, atproto.CollectionMeasurement, "subject", uri)
	attachmentURIs := findLinkedURIs(ctx, client, did, atproto.CollectionAttachment, "subjects", uri)
	evaluationURIs := findLinkedURIs(ctx, client, did, atproto.CollectionEvaluation, "subject", uri)

	totalLinked := len(measurementURIs) + len(attachmentURIs) + len(evaluationURIs)

	if !force && totalLinked > 0 {
		fmt.Fprintf(w, "Will delete activity %s and %d linked record(s):\n", extractRkey(uri), totalLinked)
		if len(measurementURIs) > 0 {
			fmt.Fprintf(w, "  %d measurement(s)\n", len(measurementURIs))
		}
		if len(attachmentURIs) > 0 {
			fmt.Fprintf(w, "  %d attachment(s)\n", len(attachmentURIs))
		}
		if len(evaluationURIs) > 0 {
			fmt.Fprintf(w, "  %d evaluation(s)\n", len(evaluationURIs))
		}
		if !menu.Confirm(w, os.Stdin, "Proceed?") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}

	// Delete linked records first, then activity
	for _, attURI := range attachmentURIs {
		attATURI, err := syntax.ParseATURI(attURI)
		if err != nil {
			continue
		}
		if err := atproto.DeleteRecord(ctx, client, did, attATURI.Collection().String(), attATURI.RecordKey().String()); err != nil {
			fmt.Fprintf(w, "  Warning: failed to delete attachment %s: %v\n", extractRkey(attURI), err)
		}
	}
	for _, mURI := range measurementURIs {
		mATURI, err := syntax.ParseATURI(mURI)
		if err != nil {
			continue
		}
		if err := atproto.DeleteRecord(ctx, client, did, mATURI.Collection().String(), mATURI.RecordKey().String()); err != nil {
			fmt.Fprintf(w, "  Warning: failed to delete measurement %s: %v\n", extractRkey(mURI), err)
		}
	}
	for _, eURI := range evaluationURIs {
		eATURI, err := syntax.ParseATURI(eURI)
		if err != nil {
			continue
		}
		if err := atproto.DeleteRecord(ctx, client, did, eATURI.Collection().String(), eATURI.RecordKey().String()); err != nil {
			fmt.Fprintf(w, "  Warning: failed to delete evaluation %s: %v\n", extractRkey(eURI), err)
		}
	}

	if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
		return fmt.Errorf("failed to delete activity: %w", err)
	}

	fmt.Fprintf(w, "Deleted activity: %s\n", extractRkey(uri))
	if len(measurementURIs) > 0 {
		fmt.Fprintf(w, "  Deleted %d linked measurement(s)\n", len(measurementURIs))
	}
	if len(attachmentURIs) > 0 {
		fmt.Fprintf(w, "  Deleted %d linked attachment(s)\n", len(attachmentURIs))
	}
	if len(evaluationURIs) > 0 {
		fmt.Fprintf(w, "  Deleted %d linked evaluation(s)\n", len(evaluationURIs))
	}
	return nil
}

// findLinkedURIs finds URIs of records that reference the target URI.
func findLinkedURIs(ctx context.Context, client *atclient.APIClient, did, collection, linkField, targetURI string) []string {
	entries, err := atproto.ListAllRecords(ctx, client, did, collection)
	if err != nil {
		return nil
	}
	var uris []string
	for _, e := range entries {
		matched := false
		if linkField == "subject" {
			if subject, ok := e.Value["subject"].(map[string]any); ok {
				if subURI, ok := subject["uri"].(string); ok && subURI == targetURI {
					matched = true
				}
			}
		} else if linkField == "subjects" {
			if subjects, ok := e.Value["subjects"].([]any); ok {
				for _, s := range subjects {
					if subMap, ok := s.(map[string]any); ok {
						if subURI, ok := subMap["uri"].(string); ok && subURI == targetURI {
							matched = true
							break
						}
					}
				}
			}
		}
		if matched {
			uris = append(uris, e.URI)
		}
	}
	return uris
}

// findCollectionsContaining finds collection URIs that contain the target activity URI in their items.
func findCollectionsContaining(ctx context.Context, client *atclient.APIClient, did, targetURI string) []string {
	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionCollection)
	if err != nil {
		return nil
	}
	var uris []string
	for _, e := range entries {
		if items, ok := e.Value["items"].([]any); ok {
			for _, item := range items {
				if itemMap, ok := item.(map[string]any); ok {
					if itemID, ok := itemMap["itemIdentifier"].(map[string]any); ok {
						if itemURI, ok := itemID["uri"].(string); ok && itemURI == targetURI {
							uris = append(uris, e.URI)
							break
						}
					}
				}
			}
		}
	}
	return uris
}

func runActivityList(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	// Build measurement count map
	measurementCounts := make(map[string]int)
	measurements, _ := atproto.ListAllRecords(ctx, client, did, atproto.CollectionMeasurement)
	for _, m := range measurements {
		if subject, ok := m.Value["subject"].(map[string]any); ok {
			if subURI, ok := subject["uri"].(string); ok {
				measurementCounts[subURI]++
			}
		}
	}

	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionActivity)
	if err != nil {
		return fmt.Errorf("failed to list activities: %w", err)
	}

	if cmd.Bool("json") {
		var records []map[string]any
		for _, e := range entries {
			entry := map[string]any{"uri": e.URI, "activity": e.Value}
			if c := measurementCounts[e.URI]; c > 0 {
				entry["measurementCount"] = c
			}
			records = append(records, entry)
		}
		fmt.Fprintln(w, prettyJSON(records))
		return nil
	}

	fmt.Fprintf(w, "\033[1m%-15s %-30s %-6s %-15s %s\033[0m\n", "ID", "TITLE", "MEAS", "SCOPE", "CREATED")
	fmt.Fprintf(w, "%-15s %-30s %-6s %-15s %s\n",
		strings.Repeat("-", 13), strings.Repeat("-", 28),
		strings.Repeat("-", 4), strings.Repeat("-", 13), strings.Repeat("-", 10))

	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		id := string(aturi.RecordKey())
		title := mapStr(e.Value, "title")
		if len(title) > 28 {
			title = title[:25] + "..."
		}

		scope := "-"
		if ws, ok := e.Value["workScope"].(map[string]any); ok {
			if s := mapStr(ws, "scope"); s != "" {
				scope = s
			}
		}
		if len(scope) > 13 {
			scope = scope[:10] + "..."
		}

		measureCount := measurementCounts[e.URI]

		created := "-"
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		fmt.Fprintf(w, "%-15s %-30s %-6d %-15s %s\n", id, title, measureCount, scope, created)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "\033[90m(no activities found)\033[0m")
	}
	return nil
}

func runActivityGet(ctx context.Context, cmd *cli.Command) error {
	arg := cmd.Args().First()
	if arg == "" {
		return fmt.Errorf("usage: hc activity get <id|at-uri>")
	}

	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	did := client.AccountDID.String()
	uri := resolveRecordURI(did, atproto.CollectionActivity, arg)

	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	activity, _, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("failed to get activity: %w", err)
	}

	result := map[string]any{
		"uri":      uri,
		"activity": activity,
	}

	// Find linked measurements
	measurementURIs := findLinkedURIs(ctx, client, did, atproto.CollectionMeasurement, "subject", uri)
	if len(measurementURIs) > 0 {
		var measurements []map[string]any
		for _, mURI := range measurementURIs {
			mATURI, err := syntax.ParseATURI(mURI)
			if err != nil {
				continue
			}
			m, _, err := atproto.GetRecord(ctx, client, did, mATURI.Collection().String(), mATURI.RecordKey().String())
			if err != nil {
				continue
			}
			m["_uri"] = mURI
			measurements = append(measurements, m)
		}
		if len(measurements) > 0 {
			result["measurements"] = measurements
		}
	}

	// Find linked attachments
	attachmentURIs := findLinkedURIs(ctx, client, did, atproto.CollectionAttachment, "subjects", uri)
	if len(attachmentURIs) > 0 {
		var attachments []map[string]any
		for _, aURI := range attachmentURIs {
			aATURI, err := syntax.ParseATURI(aURI)
			if err != nil {
				continue
			}
			a, _, err := atproto.GetRecord(ctx, client, did, aATURI.Collection().String(), aATURI.RecordKey().String())
			if err != nil {
				continue
			}
			a["_uri"] = aURI
			attachments = append(attachments, a)
		}
		if len(attachments) > 0 {
			result["attachments"] = attachments
		}
	}

	// Find linked evaluations
	evaluationURIs := findLinkedURIs(ctx, client, did, atproto.CollectionEvaluation, "subject", uri)
	if len(evaluationURIs) > 0 {
		var evaluations []map[string]any
		for _, eURI := range evaluationURIs {
			eATURI, err := syntax.ParseATURI(eURI)
			if err != nil {
				continue
			}
			e, _, err := atproto.GetRecord(ctx, client, did, eATURI.Collection().String(), eATURI.RecordKey().String())
			if err != nil {
				continue
			}
			e["_uri"] = eURI
			evaluations = append(evaluations, e)
		}
		if len(evaluations) > 0 {
			result["evaluations"] = evaluations
		}
	}

	// Find collections containing this activity
	collectionURIs := findCollectionsContaining(ctx, client, did, uri)
	if len(collectionURIs) > 0 {
		var collections []map[string]any
		for _, cURI := range collectionURIs {
			cATURI, err := syntax.ParseATURI(cURI)
			if err != nil {
				continue
			}
			c, _, err := atproto.GetRecord(ctx, client, did, cATURI.Collection().String(), cATURI.RecordKey().String())
			if err != nil {
				continue
			}
			c["_uri"] = cURI
			collections = append(collections, c)
		}
		if len(collections) > 0 {
			result["collections"] = collections
		}
	}

	fmt.Fprintln(cmd.Root().Writer, prettyJSON(result))
	return nil
}
