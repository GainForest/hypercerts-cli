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
	"github.com/GainForest/hypercerts-cli/internal/prompt"
)

var attachmentContentTypes = []string{
	"report",
	"audit",
	"evidence",
	"testimonial",
	"methodology",
}

type attachmentOption struct {
	URI          string
	Rkey         string
	Title        string
	ContentType  string
	SubjectCount int
	ContentCount int
	Created      string
}

func fetchAttachments(ctx context.Context, client *atclient.APIClient, did string) ([]attachmentOption, error) {
	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionAttachment)
	if err != nil {
		return nil, fmt.Errorf("failed to list attachments: %w", err)
	}
	var result []attachmentOption
	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		subjectCount := 0
		if subjects := mapSlice(e.Value, "subjects"); subjects != nil {
			subjectCount = len(subjects)
		}
		contentCount := 0
		if content := mapSlice(e.Value, "content"); content != nil {
			contentCount = len(content)
		}
		created := ""
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}
		result = append(result, attachmentOption{
			URI:          e.URI,
			Rkey:         string(aturi.RecordKey()),
			Title:        mapStr(e.Value, "title"),
			ContentType:  mapStr(e.Value, "contentType"),
			SubjectCount: subjectCount,
			ContentCount: contentCount,
			Created:      created,
		})
	}
	return result, nil
}

func fetchAttachmentsForActivity(ctx context.Context, client *atclient.APIClient, did, activityURI string) ([]attachmentOption, error) {
	all, err := fetchAttachments(ctx, client, did)
	if err != nil {
		return nil, err
	}

	// Need to re-fetch records to check subjects
	entries, _ := atproto.ListAllRecords(ctx, client, did, atproto.CollectionAttachment)
	uriSet := make(map[string]bool)
	for _, e := range entries {
		if subjects := mapSlice(e.Value, "subjects"); subjects != nil {
			for _, s := range subjects {
				if subMap, ok := s.(map[string]any); ok {
					if mapStr(subMap, "uri") == activityURI {
						uriSet[e.URI] = true
						break
					}
				}
			}
		}
	}

	var result []attachmentOption
	for _, a := range all {
		if uriSet[a.URI] {
			result = append(result, a)
		}
	}
	return result, nil
}

// selectSubjects allows selecting multiple activities/records as subjects.
func selectSubjects(ctx context.Context, client *atclient.APIClient, w io.Writer) ([]map[string]any, error) {
	var subjects []map[string]any
	for {
		uri, cid, err := selectActivity(ctx, client, w)
		if err != nil {
			return nil, err
		}
		subjects = append(subjects, buildStrongRef(uri, cid))

		if !menu.Confirm(w, os.Stdin, "Add another subject?") {
			break
		}
	}
	return subjects, nil
}

// promptContentURIs prompts for content URIs.
func promptContentURIs(w io.Writer) ([]map[string]any, error) {
	var content []map[string]any
	for {
		var uri string
		var err error
		if len(content) == 0 {
			uri, err = prompt.ReadRequired(w, os.Stdin, "Content URI", "URL to evidence/document")
		} else {
			uri, err = prompt.ReadOptionalField(w, os.Stdin, "Content URI", "enter to finish")
		}
		if err != nil {
			return nil, err
		}
		if uri == "" {
			break
		}
		content = append(content, map[string]any{
			"$type": "org.hypercerts.defs#uri",
			"uri":   uri,
		})

		if !menu.Confirm(w, os.Stdin, "Add another content URI?") {
			break
		}
	}
	return content, nil
}

func runAttachmentCreate(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	record := map[string]any{
		"$type":     atproto.CollectionAttachment,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	title := cmd.String("title")
	activityFlag := cmd.String("activity")
	uriFlag := cmd.String("uri")
	contentType := cmd.String("content-type")

	hasFlags := title != "" || activityFlag != "" || uriFlag != "" || contentType != ""

	if hasFlags {
		// Non-interactive: require title via flag or prompt fallback
		if title == "" {
			title, err = prompt.ReadRequired(w, os.Stdin, "Title", "max 256 chars")
			if err != nil {
				return err
			}
		}
		record["title"] = title

		// Subjects from --activity flag
		if activityFlag != "" {
			activityIDs := strings.Split(activityFlag, ",")
			var subjects []any
			for _, actID := range activityIDs {
				actID = strings.TrimSpace(actID)
				if actID == "" {
					continue
				}
				activityURI := resolveRecordURI(did, atproto.CollectionActivity, actID)
				aturi, err := syntax.ParseATURI(activityURI)
				if err != nil {
					return fmt.Errorf("invalid activity URI: %w", err)
				}
				_, activityCID, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
				if err != nil {
					return fmt.Errorf("activity not found: %s", actID)
				}
				subjects = append(subjects, buildStrongRef(activityURI, activityCID))
			}
			if len(subjects) > 0 {
				record["subjects"] = subjects
			}
		}

		// Content URIs from --uri flag
		if uriFlag != "" {
			uris := strings.Split(uriFlag, ",")
			var content []any
			for _, u := range uris {
				u = strings.TrimSpace(u)
				if u == "" {
					continue
				}
				content = append(content, map[string]any{
					"$type": "org.hypercerts.defs#uri",
					"uri":   u,
				})
			}
			if len(content) == 0 {
				return fmt.Errorf("at least one content URI is required")
			}
			record["content"] = content
		} else {
			fmt.Fprintln(w)
			content, err := promptContentURIs(w)
			if err != nil {
				return err
			}
			record["content"] = content
		}

		if contentType != "" {
			record["contentType"] = contentType
		}
	} else {
		// Interactive: show all fields at once using huh form
		var shortDesc, description string
		var addLocation bool

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Title").
					Description("Main title for this attachment").
					CharLimit(256).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("title is required")
						}
						return nil
					}).
					Value(&title),

				huh.NewSelect[string]().
					Title("Content type").
					Description("Category (optional)").
					Options(
						huh.NewOption("(skip)", ""),
						huh.NewOption("report", "report"),
						huh.NewOption("audit", "audit"),
						huh.NewOption("evidence", "evidence"),
						huh.NewOption("testimonial", "testimonial"),
						huh.NewOption("methodology", "methodology"),
					).
					Value(&contentType),

				huh.NewInput().
					Title("Short description").
					Description("Brief summary, max 300 chars (optional)").
					CharLimit(300).
					Value(&shortDesc),

				huh.NewInput().
					Title("Description").
					Description("Longer description, max 3000 chars (optional)").
					CharLimit(3000).
					Value(&description),
			).Title("Attachment Details"),

			huh.NewGroup(
				huh.NewConfirm().
					Title("Add location?").
					Inline(true).
					Value(&addLocation),
			).Title("Linked Records"),
		).WithTheme(huh.ThemeBase16())

		if err := form.Run(); err != nil {
			return err
		}

		record["title"] = title
		if contentType != "" {
			record["contentType"] = contentType
		}
		if shortDesc != "" {
			record["shortDescription"] = shortDesc
		}
		if description != "" {
			record["description"] = description
		}

		// Select subjects (activities) - needs API + menu
		fmt.Fprintln(w, "Select activities to link this attachment to:")
		subjects, err := selectSubjects(ctx, client, w)
		if err != nil {
			return err
		}
		if len(subjects) > 0 {
			record["subjects"] = subjects
		}

		// Prompt content URIs - needs loop
		fmt.Fprintln(w)
		content, err := promptContentURIs(w)
		if err != nil {
			return err
		}
		record["content"] = content

		// Handle location selection if confirmed in form
		if addLocation {
			loc, err := selectLocation(ctx, client, w)
			if err != nil {
				return err
			}
			record["location"] = buildStrongRef(loc.URI, loc.CID)
		}
	}

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionAttachment, record)
	if err != nil {
		return fmt.Errorf("failed to create attachment: %w", err)
	}

	fmt.Fprintf(w, "\n\033[32m✓\033[0m Created attachment: %s\n", uri)
	return nil
}

func runAttachmentEdit(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	arg := cmd.Args().First()
	var uri string
	if arg == "" {
		attachments, err := fetchAttachments(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.SingleSelect(w, attachments, "attachment",
			func(a attachmentOption) string { return a.Title },
			func(a attachmentOption) string {
				if a.ContentType != "" {
					return a.ContentType
				}
				return a.Rkey
			},
		)
		if err != nil {
			return err
		}
		uri = selected.URI
	} else {
		uri = resolveRecordURI(did, atproto.CollectionAttachment, arg)
	}

	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	existing, cid, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("attachment not found: %s", extractRkey(uri))
	}

	// Get current values
	currentTitle := mapStr(existing, "title")
	currentContentType := mapStr(existing, "contentType")
	currentShortDesc := mapStr(existing, "shortDescription")
	currentDesc := mapStr(existing, "description")

	// Get new values from flags or prompts
	newTitle := cmd.String("title")
	newContentType := cmd.String("content-type")

	// Track changes
	changed := false
	isInteractive := newTitle == "" && newContentType == ""

	if isInteractive {
		// Interactive mode
		newTitle, err = prompt.ReadLineWithDefault(w, os.Stdin, "Title", "required", currentTitle)
		if err != nil {
			return err
		}

		fmt.Fprintln(w)
		if menu.Confirm(w, os.Stdin, "Change content type?") {
			selected, err := menu.SingleSelect(w, attachmentContentTypes, "content type",
				func(s string) string { return s },
				func(s string) string { return "" },
			)
			if err == nil {
				newContentType = *selected
			}
		}

		fmt.Fprintln(w)
		if menu.Confirm(w, os.Stdin, "Edit descriptions?") {
			newShortDesc, err := prompt.ReadLineWithDefault(w, os.Stdin, "Short description", "max 300 chars", currentShortDesc)
			if err != nil {
				return err
			}
			if newShortDesc != currentShortDesc {
				existing["shortDescription"] = newShortDesc
				changed = true
			}

			newDesc, err := prompt.ReadLineWithDefault(w, os.Stdin, "Description", "max 3000 chars", currentDesc)
			if err != nil {
				return err
			}
			if newDesc != currentDesc {
				existing["description"] = newDesc
				changed = true
			}
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
	}
	if newTitle == "" {
		newTitle = currentTitle
	}
	if newTitle != currentTitle {
		existing["title"] = newTitle
		changed = true
	}
	if newContentType != "" && newContentType != currentContentType {
		existing["contentType"] = newContentType
		changed = true
	}

	if !changed {
		fmt.Fprintln(w, "No changes.")
		return nil
	}

	resultURI, err := atproto.PutRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String(), existing, &cid)
	if err != nil {
		return fmt.Errorf("failed to update attachment: %w", err)
	}

	fmt.Fprintf(w, "\033[32m✓\033[0m Updated attachment: %s\n", resultURI)
	return nil
}

func runAttachmentDelete(ctx context.Context, cmd *cli.Command) error {
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
		attachments, err := fetchAttachments(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.MultiSelect(w, attachments, "attachment",
			func(a attachmentOption) string { return a.Title },
			func(a attachmentOption) string {
				info := a.Rkey
				if a.ContentType != "" {
					info = a.ContentType + " | " + info
				}
				return info
			},
		)
		if err != nil {
			return err
		}
		if !menu.ConfirmBulkDelete(w, os.Stdin, len(selected), "attachment") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		for _, a := range selected {
			aturi, _ := syntax.ParseATURI(a.URI)
			if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
				fmt.Fprintf(w, "  Warning: %v\n", err)
			} else {
				fmt.Fprintf(w, "Deleted attachment: %s\n", a.Rkey)
			}
		}
		return nil
	}

	uri := resolveRecordURI(did, atproto.CollectionAttachment, id)
	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, fmt.Sprintf("Delete attachment %s?", extractRkey(uri))) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
		return fmt.Errorf("failed to delete attachment: %w", err)
	}
	fmt.Fprintf(w, "Deleted attachment: %s\n", extractRkey(uri))
	return nil
}

func runAttachmentList(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	did := client.AccountDID.String()

	// Filter by activity if specified
	activityFilter := cmd.String("activity")
	var entries []attachmentOption
	if activityFilter != "" {
		activityURI := resolveRecordURI(did, atproto.CollectionActivity, activityFilter)
		entries, err = fetchAttachmentsForActivity(ctx, client, did, activityURI)
	} else {
		entries, err = fetchAttachments(ctx, client, did)
	}
	if err != nil {
		return err
	}

	if cmd.Bool("json") {
		// Re-fetch full records for JSON output
		rawEntries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionAttachment)
		if err != nil {
			return fmt.Errorf("failed to list attachments: %w", err)
		}
		var records []map[string]any

		// Build URI set for filtered entries
		uriSet := make(map[string]bool)
		for _, e := range entries {
			uriSet[e.URI] = true
		}

		for _, e := range rawEntries {
			if activityFilter != "" && !uriSet[e.URI] {
				continue
			}
			records = append(records, map[string]any{"uri": e.URI, "record": e.Value})
		}
		fmt.Fprintln(w, prettyJSON(records))
		return nil
	}

	fmt.Fprintf(w, "\033[1m%-15s %-25s %-12s %-8s %-8s %s\033[0m\n", "ID", "TITLE", "TYPE", "SUBJECTS", "CONTENT", "CREATED")
	fmt.Fprintf(w, "%-15s %-25s %-12s %-8s %-8s %s\n",
		strings.Repeat("-", 13), strings.Repeat("-", 23),
		strings.Repeat("-", 10), strings.Repeat("-", 6),
		strings.Repeat("-", 6), strings.Repeat("-", 10))

	for _, a := range entries {
		title := a.Title
		if len(title) > 23 {
			title = title[:20] + "..."
		}
		contentType := a.ContentType
		if contentType == "" {
			contentType = "-"
		}
		if len(contentType) > 10 {
			contentType = contentType[:7] + "..."
		}

		fmt.Fprintf(w, "%-15s %-25s %-12s %-8d %-8d %s\n",
			a.Rkey, title, contentType, a.SubjectCount, a.ContentCount, a.Created)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "\033[90m(no attachments found)\033[0m")
	}
	return nil
}

func runAttachmentGet(ctx context.Context, cmd *cli.Command) error {
	return runSimpleGet(ctx, cmd, atproto.CollectionAttachment, "attachment")
}
