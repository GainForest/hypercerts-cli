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

type rightsOption struct {
	URI         string
	CID         string
	Rkey        string
	Name        string
	Type        string
	Description string
	Created     string
}

func fetchRights(ctx context.Context, client *atclient.APIClient, did string) ([]rightsOption, error) {
	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionRights)
	if err != nil {
		return nil, fmt.Errorf("failed to list rights: %w", err)
	}
	var result []rightsOption
	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		created := ""
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}
		result = append(result, rightsOption{
			URI:         e.URI,
			CID:         e.CID,
			Rkey:        string(aturi.RecordKey()),
			Name:        mapStr(e.Value, "rightsName"),
			Type:        mapStr(e.Value, "rightsType"),
			Description: mapStr(e.Value, "rightsDescription"),
			Created:     created,
		})
	}
	return result, nil
}

// selectRights shows a menu to select existing rights or create new ones.
// Returns the selected rights option with URI and CID for strongRef.
func selectRights(ctx context.Context, client *atclient.APIClient, w io.Writer) (*rightsOption, error) {
	did := client.AccountDID.String()
	rights, err := fetchRights(ctx, client, did)
	if err != nil {
		return nil, err
	}

	selected, isCreate, err := menu.SingleSelectWithCreate(w, rights, "rights",
		func(r rightsOption) string {
			if r.Name != "" {
				return r.Name
			}
			return r.Rkey
		},
		func(r rightsOption) string {
			if r.Type != "" {
				return r.Type
			}
			return ""
		},
		"Create new rights...",
	)
	if err != nil {
		return nil, err
	}

	if isCreate {
		return createRightsInline(ctx, client, w)
	}

	// Fetch CID if not present (list doesn't always include it)
	if selected.CID == "" {
		aturi, _ := syntax.ParseATURI(selected.URI)
		_, cid, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
		if err == nil {
			selected.CID = cid
		}
	}

	return selected, nil
}

func createRightsInline(ctx context.Context, client *atclient.APIClient, w io.Writer) (*rightsOption, error) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  \033[1mNew Rights\033[0m")

	name, err := prompt.ReadRequired(w, os.Stdin, "  Rights name", "max 100 chars")
	if err != nil {
		return nil, err
	}

	rightsType, err := prompt.ReadRequired(w, os.Stdin, "  Rights type", "short ID max 10 chars, e.g. CC-BY-4.0")
	if err != nil {
		return nil, err
	}

	description, err := prompt.ReadRequired(w, os.Stdin, "  Description", "")
	if err != nil {
		return nil, err
	}

	record := map[string]any{
		"$type":             atproto.CollectionRights,
		"rightsName":        name,
		"rightsType":        rightsType,
		"rightsDescription": description,
		"createdAt":         time.Now().UTC().Format(time.RFC3339),
	}

	uri, cid, err := atproto.CreateRecord(ctx, client, atproto.CollectionRights, record)
	if err != nil {
		return nil, fmt.Errorf("failed to create rights: %w", err)
	}

	fmt.Fprintf(w, "  \033[32m✓\033[0m Created rights: %s\n", uri)
	return &rightsOption{
		URI:         uri,
		CID:         cid,
		Rkey:        extractRkey(uri),
		Name:        name,
		Type:        rightsType,
		Description: description,
		Created:     time.Now().Format("2006-01-02"),
	}, nil
}

func runRightsCreate(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer

	record := map[string]any{
		"$type":     atproto.CollectionRights,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	name := cmd.String("name")
	rightsType := cmd.String("type")
	description := cmd.String("description")
	attachmentURI := cmd.String("attachment")

	if name == "" && rightsType == "" && description == "" {
		// Interactive mode: show all fields at once using huh form
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Rights name").
					Description("max 100 chars").
					CharLimit(100).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("rights name is required")
						}
						return nil
					}).
					Value(&name),

				huh.NewInput().
					Title("Rights type").
					Description("short ID max 10 chars, e.g. CC-BY-4.0").
					CharLimit(10).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("rights type is required")
						}
						return nil
					}).
					Value(&rightsType),

				huh.NewInput().
					Title("Description").
					Description("Describe the rights").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("description is required")
						}
						return nil
					}).
					Value(&description),

				huh.NewInput().
					Title("Attachment URI").
					Description("URL to legal document (optional)").
					Value(&attachmentURI),
			).Title("Rights Details"),
		).WithTheme(huh.ThemeBase16())

		if err := form.Run(); err != nil {
			return err
		}
	} else {
		// Non-interactive: some flags provided, prompt for any missing required fields
		if name == "" {
			name, err = prompt.ReadRequired(w, os.Stdin, "Rights name", "max 100 chars")
			if err != nil {
				return err
			}
		}
		if rightsType == "" {
			rightsType, err = prompt.ReadRequired(w, os.Stdin, "Rights type", "short ID max 10 chars, e.g. CC-BY-4.0")
			if err != nil {
				return err
			}
		}
		if description == "" {
			description, err = prompt.ReadRequired(w, os.Stdin, "Description", "")
			if err != nil {
				return err
			}
		}
	}

	record["rightsName"] = name
	record["rightsType"] = rightsType
	record["rightsDescription"] = description

	if attachmentURI != "" {
		record["attachment"] = map[string]any{
			"$type": "org.hypercerts.defs#uri",
			"uri":   attachmentURI,
		}
	}

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionRights, record)
	if err != nil {
		return fmt.Errorf("failed to create rights: %w", err)
	}

	fmt.Fprintf(w, "\n\033[32m✓\033[0m Created rights: %s\n", uri)
	return nil
}

func runRightsEdit(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	arg := cmd.Args().First()
	var uri string
	if arg == "" {
		rights, err := fetchRights(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.SingleSelect(w, rights, "rights",
			func(r rightsOption) string { return r.Name },
			func(r rightsOption) string { return r.Type },
		)
		if err != nil {
			return err
		}
		uri = selected.URI
	} else {
		uri = resolveRecordURI(did, atproto.CollectionRights, arg)
	}

	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	existing, cid, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("rights not found: %s", extractRkey(uri))
	}

	// Get current values
	currentName := mapStr(existing, "rightsName")
	currentType := mapStr(existing, "rightsType")
	currentDesc := mapStr(existing, "rightsDescription")

	// Get new values from flags or prompts
	newName := cmd.String("name")
	newType := cmd.String("type")
	newDesc := cmd.String("description")

	// Apply defaults and track changes
	changed := false
	isInteractive := newName == "" && newType == "" && newDesc == ""

	if isInteractive {
		// Interactive mode
		newName, err = prompt.ReadLineWithDefault(w, os.Stdin, "Rights name", "required", currentName)
		if err != nil {
			return err
		}
		newType, err = prompt.ReadLineWithDefault(w, os.Stdin, "Rights type", "required", currentType)
		if err != nil {
			return err
		}
		newDesc, err = prompt.ReadLineWithDefault(w, os.Stdin, "Description", "required", currentDesc)
		if err != nil {
			return err
		}

		// Optional attachment
		existingAttachment := mapMap(existing, "attachment")
		attachmentLabel := "Add attachment URI?"
		currentAttachmentURI := ""
		if existingAttachment != nil {
			currentAttachmentURI = mapStr(existingAttachment, "uri")
			if currentAttachmentURI != "" {
				attachmentLabel = "Replace attachment URI?"
			}
		}
		fmt.Fprintln(w)
		if menu.Confirm(w, os.Stdin, attachmentLabel) {
			newAttachment, err := prompt.ReadLineWithDefault(w, os.Stdin, "Attachment URI", "URL to legal document", currentAttachmentURI)
			if err != nil {
				return err
			}
			if newAttachment != "" && newAttachment != currentAttachmentURI {
				existing["attachment"] = map[string]any{
					"$type": "org.hypercerts.defs#uri",
					"uri":   newAttachment,
				}
				changed = true
			}
		}
	}

	if newName == "" {
		newName = currentName
	}
	if newType == "" {
		newType = currentType
	}
	if newDesc == "" {
		newDesc = currentDesc
	}

	if newName != currentName {
		existing["rightsName"] = newName
		changed = true
	}
	if newType != currentType {
		existing["rightsType"] = newType
		changed = true
	}
	if newDesc != currentDesc {
		existing["rightsDescription"] = newDesc
		changed = true
	}

	if !changed {
		fmt.Fprintln(w, "No changes.")
		return nil
	}

	resultURI, err := atproto.PutRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String(), existing, &cid)
	if err != nil {
		return fmt.Errorf("failed to update rights: %w", err)
	}

	fmt.Fprintf(w, "\033[32m✓\033[0m Updated rights: %s\n", resultURI)
	return nil
}

func runRightsDelete(ctx context.Context, cmd *cli.Command) error {
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
		rights, err := fetchRights(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.MultiSelect(w, rights, "rights",
			func(r rightsOption) string { return r.Name },
			func(r rightsOption) string { return r.Type },
		)
		if err != nil {
			return err
		}
		if !menu.ConfirmBulkDelete(w, os.Stdin, len(selected), "rights") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		for _, r := range selected {
			aturi, _ := syntax.ParseATURI(r.URI)
			if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
				fmt.Fprintf(w, "  Warning: %v\n", err)
			} else {
				fmt.Fprintf(w, "Deleted rights: %s\n", r.Rkey)
			}
		}
		return nil
	}

	uri := resolveRecordURI(did, atproto.CollectionRights, id)
	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, fmt.Sprintf("Delete rights %s?", extractRkey(uri))) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
		return fmt.Errorf("failed to delete rights: %w", err)
	}
	fmt.Fprintf(w, "Deleted rights: %s\n", extractRkey(uri))
	return nil
}

func runRightsList(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	did := client.AccountDID.String()

	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionRights)
	if err != nil {
		return fmt.Errorf("failed to list rights: %w", err)
	}

	if cmd.Bool("json") {
		var records []map[string]any
		for _, e := range entries {
			records = append(records, map[string]any{"uri": e.URI, "record": e.Value})
		}
		fmt.Fprintln(w, prettyJSON(records))
		return nil
	}

	fmt.Fprintf(w, "\033[1m%-15s %-25s %-12s %-35s %s\033[0m\n", "ID", "NAME", "TYPE", "DESCRIPTION", "CREATED")
	fmt.Fprintf(w, "%-15s %-25s %-12s %-35s %s\n",
		strings.Repeat("-", 13), strings.Repeat("-", 23),
		strings.Repeat("-", 10), strings.Repeat("-", 33), strings.Repeat("-", 10))

	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		id := string(aturi.RecordKey())
		name := mapStr(e.Value, "rightsName")
		rightsType := mapStr(e.Value, "rightsType")
		description := mapStr(e.Value, "rightsDescription")

		if len(name) > 23 {
			name = name[:20] + "..."
		}
		if len(rightsType) > 10 {
			rightsType = rightsType[:7] + "..."
		}
		if len(description) > 33 {
			description = description[:30] + "..."
		}

		created := "-"
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		fmt.Fprintf(w, "%-15s %-25s %-12s %-35s %s\n", id, name, rightsType, description, created)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "\033[90m(no rights found)\033[0m")
	}
	return nil
}

func runRightsGet(ctx context.Context, cmd *cli.Command) error {
	return runSimpleGet(ctx, cmd, atproto.CollectionRights, "rights")
}
