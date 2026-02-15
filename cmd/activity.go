package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v3"

	"github.com/GainForest/hypercerts-cli/internal/atproto"
	"github.com/GainForest/hypercerts-cli/internal/menu"
	"github.com/GainForest/hypercerts-cli/internal/style"
)

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

	// Check for non-interactive mode (flags provided)
	title := cmd.String("title")
	shortDesc := cmd.String("description")
	hasFlags := title != "" || shortDesc != ""

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

	if hasFlags {
		// Non-interactive: require title and description via flags or prompt fallback
		if title == "" {
			err = huh.NewInput().Title("Title").Description("max 256 chars").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("title is required")
					}
					return nil
				}).Value(&title).WithTheme(style.Theme()).Run()
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return fmt.Errorf("cancelled")
				}
				return err
			}
		}
		if shortDesc == "" {
			err = huh.NewInput().Title("Short description").Description("max 300 chars").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("short description is required")
					}
					return nil
				}).Value(&shortDesc).WithTheme(style.Theme()).Run()
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return fmt.Errorf("cancelled")
				}
				return err
			}
		}
		record["title"] = title
		record["shortDescription"] = shortDesc
	} else {
		// Interactive: bubbletea form with live preview card
		result, err := runActivityForm()
		if err != nil {
			if err == huh.ErrUserAborted {
				return nil
			}
			return err
		}

		record["title"] = result.Title
		record["shortDescription"] = result.ShortDesc

		if result.Description != "" {
			record["description"] = result.Description
		}
		if result.WorkScope != "" {
			record["workScope"] = map[string]any{
				"$type": atproto.CollectionActivity + "#workScopeString",
				"scope": result.WorkScope,
			}
		}
		if result.StartDate != "" {
			record["startDate"] = normalizeDate(result.StartDate)
		}
		if result.EndDate != "" {
			record["endDate"] = normalizeDate(result.EndDate)
		}
		if result.ImageURI != "" {
			record["image"] = map[string]any{
				"$type": "org.hypercerts.defs#uri",
				"uri":   result.ImageURI,
			}
		}

		// Handle linked records interactively (need API calls, run after bubbletea exits)
		if result.AddContributors {
			contributors, err := promptContributors(ctx, client, w)
			if err != nil {
				return err
			}
			if len(contributors) > 0 {
				record["contributors"] = contributors
			}
		}
		if result.AddLocations {
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
		}
		if result.AddRights {
			rights, err := selectRights(ctx, client, w)
			if err != nil {
				return err
			}
			record["rights"] = buildStrongRef(rights.URI, rights.CID)
		}
	}

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionActivity, record)
	if err != nil {
		return fmt.Errorf("failed to create activity: %w", err)
	}

	fmt.Fprintf(w, "\n\033[32m✓\033[0m Created activity: %s\n", uri)
	return nil
}
func promptContributors(ctx context.Context, client *atclient.APIClient, w io.Writer) ([]any, error) {
	var contributors []any
	for i := 1; ; i++ {
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

		var role, weight string
		var addAnother bool

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().Title("Role").Description("optional").Value(&role),
				huh.NewInput().Title("Weight").Description("optional, numeric").Value(&weight),
				huh.NewConfirm().Title("Add another contributor?").Inline(true).Value(&addAnother),
			).Title(fmt.Sprintf("Contributor %d", i)),
		).WithTheme(style.Theme())

		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil, fmt.Errorf("cancelled")
			}
			return nil, err
		}

		if role != "" {
			contribObj["contributionDetails"] = map[string]any{
				"$type": atproto.CollectionActivity + "#contributorRole",
				"role":  role,
			}
		}
		if weight != "" {
			contribObj["contributionWeight"] = weight
		}

		contributors = append(contributors, contribObj)
		fmt.Fprintf(w, "\n")

		if !addAnother {
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
		currentTitle := mapStr(existing, "title")
		currentDesc := mapStr(existing, "shortDescription")
		currentStart := mapStr(existing, "startDate")
		currentEnd := mapStr(existing, "endDate")

		newTitle := currentTitle
		newDesc := currentDesc
		newStart := currentStart
		newEnd := currentEnd

		existingImage := mapMap(existing, "image")
		currentImageURI := ""
		if existingImage != nil {
			currentImageURI = mapStr(existingImage, "uri")
		}
		newImageURI := currentImageURI

		var editLocations, editRights bool

		existingLocs := mapSlice(existing, "locations")
		locTitle := "Link locations?"
		if len(existingLocs) > 0 {
			locTitle = fmt.Sprintf("Replace %d location(s)?", len(existingLocs))
		}
		existingRights := mapMap(existing, "rights")
		rightsTitle := "Link rights?"
		if existingRights != nil {
			rightsTitle = "Replace rights?"
		}

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Title").
					Description("Main title for this hypercert").
					CharLimit(256).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("title is required")
						}
						return nil
					}).
					Value(&newTitle),

				huh.NewInput().
					Title("Short description").
					Description("Brief summary of the activity").
					CharLimit(300).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("short description is required")
						}
						return nil
					}).
					Value(&newDesc),
			).Title("Activity Details"),

			huh.NewGroup(
				huh.NewInput().
					Title("Start date").
					Description("YYYY-MM-DD").
					Value(&newStart),

				huh.NewInput().
					Title("End date").
					Description("YYYY-MM-DD").
					Value(&newEnd),

				huh.NewInput().
					Title("Image URI").
					Description("URL to hypercert image").
					Value(&newImageURI),
			).Title("Dates & Media"),

			huh.NewGroup(
				huh.NewConfirm().
					Title(locTitle).
					Description("Geographic coordinates for this activity").
					Value(&editLocations),

				huh.NewConfirm().
					Title(rightsTitle).
					Description("License or rights for this claim").
					Value(&editRights),
			).Title("Linked Records"),
		).WithTheme(style.Theme())

		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return fmt.Errorf("cancelled")
			}
			return err
		}

		if newTitle != currentTitle {
			existing["title"] = newTitle
			changed = true
		}
		if newDesc != currentDesc {
			existing["shortDescription"] = newDesc
			changed = true
		}
		if newStart != "" && newStart != currentStart {
			existing["startDate"] = normalizeDate(newStart)
			changed = true
		}
		if newEnd != "" && newEnd != currentEnd {
			existing["endDate"] = normalizeDate(newEnd)
			changed = true
		}
		if newImageURI != "" && newImageURI != currentImageURI {
			existing["image"] = map[string]any{
				"$type": "org.hypercerts.defs#uri",
				"uri":   newImageURI,
			}
			changed = true
		}

		if editLocations {
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

		if editRights {
			rights, err := selectRights(ctx, client, w)
			if err != nil {
				return err
			}
			existing["rights"] = buildStrongRef(rights.URI, rights.CID)
			changed = true
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

	w := cmd.Root().Writer
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

	showMeasurements := cmd.Bool("measurements")
	showAttachments := cmd.Bool("attachments")
	showEvaluations := cmd.Bool("evaluations")
	showAll := cmd.Bool("all")
	useJSON := cmd.Bool("json")

	if showAll {
		showMeasurements = true
		showAttachments = true
		showEvaluations = true
	}

	// If no backlink flags, show activity JSON and summary
	showBacklinks := showMeasurements || showAttachments || showEvaluations

	if !showBacklinks {
		// Default: show activity record as JSON
		result := map[string]any{
			"uri":      uri,
			"activity": activity,
		}

		// Fetch backlinks summary to show counts
		summary, err := atproto.GetAllBacklinks(ctx, uri)
		if err == nil && summary != nil {
			counts := map[string]int{}
			for collection, paths := range summary.Links {
				for _, c := range paths {
					counts[collection] += c.Records
				}
			}
			if len(counts) > 0 {
				result["backlinks"] = counts
			}
		}

		fmt.Fprintln(w, prettyJSON(result))
		return nil
	}

	// Show activity header
	title := mapStr(activity, "title")
	desc := mapStr(activity, "shortDescription")
	fmt.Fprintf(w, "\033[1m%s\033[0m\n", title)
	if desc != "" {
		fmt.Fprintf(w, "\033[90m%s\033[0m\n", desc)
	}
	fmt.Fprintf(w, "URI: %s\n", uri)
	fmt.Fprintln(w)

	// Fetch backlinked records via Constellation
	if showMeasurements {
		records, err := atproto.GetAllBacklinkRecords(ctx, uri, atproto.CollectionMeasurement, ".subject.uri")
		if err != nil {
			fmt.Fprintf(w, "\033[33mWarning: failed to fetch measurement backlinks: %v\033[0m\n", err)
		} else {
			fmt.Fprintf(w, "\033[1mMeasurements (%d)\033[0m\n", len(records))
			if len(records) == 0 {
				fmt.Fprintln(w, "\033[90m  (none)\033[0m")
			} else if useJSON {
				printBacklinkRecordsJSON(ctx, client, w, records)
			} else {
				fmt.Fprintf(w, "  %-15s %-12s %-20s %-10s %-10s %s\n", "ID", "DID", "METRIC", "VALUE", "UNIT", "CREATED")
				fmt.Fprintf(w, "  %-15s %-12s %-20s %-10s %-10s %s\n",
					strings.Repeat("-", 13), strings.Repeat("-", 10),
					strings.Repeat("-", 18), strings.Repeat("-", 8),
					strings.Repeat("-", 8), strings.Repeat("-", 10))
				for _, lr := range records {
					rec, _, err := atproto.GetRecord(ctx, client, lr.DID, lr.Collection, lr.Rkey)
					if err != nil {
						fmt.Fprintf(w, "  %-15s %-12s \033[90m(failed to fetch)\033[0m\n", lr.Rkey, truncate(lr.DID, 10))
						continue
					}
					metric := truncate(mapStr(rec, "metric"), 18)
					value := truncate(mapStr(rec, "value"), 8)
					unit := truncate(mapStr(rec, "unit"), 8)
					created := formatDate(mapStr(rec, "createdAt"))
					didShort := truncate(lr.DID, 10)
					fmt.Fprintf(w, "  %-15s %-12s %-20s %-10s %-10s %s\n", lr.Rkey, didShort, metric, value, unit, created)
				}
			}
			fmt.Fprintln(w)
		}
	}

	if showAttachments {
		// Attachments can link via .subjects[].uri or .subject.uri
		records, err := atproto.GetAllBacklinkRecords(ctx, uri, atproto.CollectionAttachment, ".subjects[].uri")
		if err != nil {
			// Try alternate path
			records, err = atproto.GetAllBacklinkRecords(ctx, uri, atproto.CollectionAttachment, ".subject.uri")
		}
		if err != nil {
			fmt.Fprintf(w, "\033[33mWarning: failed to fetch attachment backlinks: %v\033[0m\n", err)
		} else {
			fmt.Fprintf(w, "\033[1mAttachments (%d)\033[0m\n", len(records))
			if len(records) == 0 {
				fmt.Fprintln(w, "\033[90m  (none)\033[0m")
			} else if useJSON {
				printBacklinkRecordsJSON(ctx, client, w, records)
			} else {
				fmt.Fprintf(w, "  %-15s %-12s %-25s %-12s %s\n", "ID", "DID", "TITLE", "TYPE", "CREATED")
				fmt.Fprintf(w, "  %-15s %-12s %-25s %-12s %s\n",
					strings.Repeat("-", 13), strings.Repeat("-", 10),
					strings.Repeat("-", 23), strings.Repeat("-", 10),
					strings.Repeat("-", 10))
				for _, lr := range records {
					rec, _, err := atproto.GetRecord(ctx, client, lr.DID, lr.Collection, lr.Rkey)
					if err != nil {
						fmt.Fprintf(w, "  %-15s %-12s \033[90m(failed to fetch)\033[0m\n", lr.Rkey, truncate(lr.DID, 10))
						continue
					}
					title := truncate(mapStr(rec, "title"), 23)
					contentType := truncate(mapStr(rec, "contentType"), 10)
					if contentType == "" {
						contentType = "-"
					}
					created := formatDate(mapStr(rec, "createdAt"))
					didShort := truncate(lr.DID, 10)
					fmt.Fprintf(w, "  %-15s %-12s %-25s %-12s %s\n", lr.Rkey, didShort, title, contentType, created)
				}
			}
			fmt.Fprintln(w)
		}
	}

	if showEvaluations {
		records, err := atproto.GetAllBacklinkRecords(ctx, uri, atproto.CollectionEvaluation, ".subject.uri")
		if err != nil {
			fmt.Fprintf(w, "\033[33mWarning: failed to fetch evaluation backlinks: %v\033[0m\n", err)
		} else {
			fmt.Fprintf(w, "\033[1mEvaluations (%d)\033[0m\n", len(records))
			if len(records) == 0 {
				fmt.Fprintln(w, "\033[90m  (none)\033[0m")
			} else if useJSON {
				printBacklinkRecordsJSON(ctx, client, w, records)
			} else {
				fmt.Fprintf(w, "  %-15s %-12s %-35s %-10s %s\n", "ID", "DID", "SUMMARY", "SCORE", "CREATED")
				fmt.Fprintf(w, "  %-15s %-12s %-35s %-10s %s\n",
					strings.Repeat("-", 13), strings.Repeat("-", 10),
					strings.Repeat("-", 33), strings.Repeat("-", 8),
					strings.Repeat("-", 10))
				for _, lr := range records {
					rec, _, err := atproto.GetRecord(ctx, client, lr.DID, lr.Collection, lr.Rkey)
					if err != nil {
						fmt.Fprintf(w, "  %-15s %-12s \033[90m(failed to fetch)\033[0m\n", lr.Rkey, truncate(lr.DID, 10))
						continue
					}
					summary := truncate(mapStr(rec, "summary"), 33)
					scoreStr := "-"
					if score := mapMap(rec, "score"); score != nil {
						if v, ok := score["value"].(float64); ok {
							if m, ok := score["max"].(float64); ok {
								scoreStr = fmt.Sprintf("%d/%d", int(v), int(m))
							}
						}
					}
					created := formatDate(mapStr(rec, "createdAt"))
					didShort := truncate(lr.DID, 10)
					fmt.Fprintf(w, "  %-15s %-12s %-35s %-10s %s\n", lr.Rkey, didShort, summary, scoreStr, created)
				}
			}
			fmt.Fprintln(w)
		}
	}

	return nil
}

// printBacklinkRecordsJSON fetches and prints full records as JSON.
func printBacklinkRecordsJSON(ctx context.Context, client *atclient.APIClient, w io.Writer, records []atproto.LinkingRecord) {
	var results []map[string]any
	for _, lr := range records {
		rec, _, err := atproto.GetRecord(ctx, client, lr.DID, lr.Collection, lr.Rkey)
		if err != nil {
			continue
		}
		rec["_uri"] = fmt.Sprintf("at://%s/%s/%s", lr.DID, lr.Collection, lr.Rkey)
		rec["_did"] = lr.DID
		results = append(results, rec)
	}
	fmt.Fprintln(w, prettyJSON(results))
}

// truncate shortens a string to max length with "..." suffix.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// formatDate converts RFC3339 to YYYY-MM-DD, or returns "-".
func formatDate(s string) string {
	if s == "" {
		return "-"
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Format("2006-01-02")
	}
	return s
}
