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

type contributorOption struct {
	URI         string
	CID         string
	Identifier  string
	DisplayName string
}

func fetchContributors(ctx context.Context, client *atclient.APIClient) ([]contributorOption, error) {
	entries, err := atproto.ListAllRecords(ctx, client, client.AccountDID.String(), atproto.CollectionContributorInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to list contributors: %w", err)
	}
	var result []contributorOption
	for _, e := range entries {
		result = append(result, contributorOption{
			URI:         e.URI,
			CID:         e.CID,
			Identifier:  mapStr(e.Value, "identifier"),
			DisplayName: mapStr(e.Value, "displayName"),
		})
	}
	return result, nil
}

func runContributorList(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	did := client.AccountDID.String()

	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionContributorInfo)
	if err != nil {
		return fmt.Errorf("failed to list contributors: %w", err)
	}

	if cmd.Bool("json") {
		var records []map[string]any
		for _, e := range entries {
			records = append(records, map[string]any{"uri": e.URI, "record": e.Value})
		}
		fmt.Fprintln(w, prettyJSON(records))
		return nil
	}

	fmt.Fprintf(w, "\033[1m%-15s %-30s %-25s %s\033[0m\n", "ID", "IDENTIFIER", "NAME", "CREATED")
	fmt.Fprintf(w, "%-15s %-30s %-25s %s\n",
		strings.Repeat("-", 13), strings.Repeat("-", 28),
		strings.Repeat("-", 23), strings.Repeat("-", 10))

	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		id := string(aturi.RecordKey())
		identifier := mapStr(e.Value, "identifier")
		displayName := mapStr(e.Value, "displayName")
		if len(identifier) > 28 {
			identifier = identifier[:25] + "..."
		}
		if len(displayName) > 23 {
			displayName = displayName[:20] + "..."
		}
		created := "-"
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}
		fmt.Fprintf(w, "%-15s %-30s %-25s %s\n", id, identifier, displayName, created)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "\033[90m(no contributors found)\033[0m")
	}
	return nil
}

func runContributorCreate(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer

	identifier := cmd.String("identifier")
	displayName := cmd.String("name")

	if identifier == "" {
		identifier, err = prompt.ReadLineWithDefault(w, os.Stdin, "Identifier", "DID or profile URI", "")
		if err != nil {
			return err
		}
		if identifier == "" {
			return fmt.Errorf("identifier is required")
		}
	}
	if displayName == "" {
		displayName, err = prompt.ReadOptionalField(w, os.Stdin, "Display name", "max 100 chars, optional")
		if err != nil {
			return err
		}
	}

	record := map[string]any{
		"$type":     atproto.CollectionContributorInfo,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}
	if identifier != "" {
		record["identifier"] = identifier
	}
	if displayName != "" {
		if len(displayName) > 100 {
			return fmt.Errorf("display name exceeds 100 characters (%d)", len(displayName))
		}
		record["displayName"] = displayName
	}

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionContributorInfo, record)
	if err != nil {
		return fmt.Errorf("failed to create contributor: %w", err)
	}

	fmt.Fprintf(w, "\033[32m\u2713\033[0m Created contributor: %s\n", uri)
	return nil
}

func runContributorEdit(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	arg := cmd.Args().First()
	var uri string
	if arg == "" {
		contributors, err := fetchContributors(ctx, client)
		if err != nil {
			return err
		}
		selected, err := menu.SingleSelect(w, contributors, "contributor",
			func(c contributorOption) string {
				if c.DisplayName != "" {
					return c.DisplayName
				}
				return c.Identifier
			},
			func(c contributorOption) string {
				if c.DisplayName != "" {
					return c.Identifier
				}
				return ""
			},
		)
		if err != nil {
			return err
		}
		uri = selected.URI
	} else {
		uri = resolveRecordURI(did, atproto.CollectionContributorInfo, arg)
	}

	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	existing, cid, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("contributor not found: %s", extractRkey(uri))
	}

	newIdentifier := cmd.String("identifier")
	newName := cmd.String("name")

	if newIdentifier == "" && newName == "" {
		currentID := mapStr(existing, "identifier")
		currentName := mapStr(existing, "displayName")

		newIdentifier, err = prompt.ReadLineWithDefault(w, os.Stdin, "Identifier", "DID or profile URI", currentID)
		if err != nil {
			return err
		}
		newName, err = prompt.ReadLineWithDefault(w, os.Stdin, "Display name", "max 100 chars", currentName)
		if err != nil {
			return err
		}
	}

	changed := false
	if newIdentifier != "" && newIdentifier != mapStr(existing, "identifier") {
		existing["identifier"] = newIdentifier
		changed = true
	}
	if newName != "" && newName != mapStr(existing, "displayName") {
		if len(newName) > 100 {
			return fmt.Errorf("display name exceeds 100 characters (%d)", len(newName))
		}
		existing["displayName"] = newName
		changed = true
	}

	if !changed {
		fmt.Fprintln(w, "No changes.")
		return nil
	}

	resultURI, err := atproto.PutRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String(), existing, &cid)
	if err != nil {
		return fmt.Errorf("failed to update contributor: %w", err)
	}

	fmt.Fprintf(w, "\033[32m\u2713\033[0m Updated contributor: %s\n", resultURI)
	return nil
}

func runContributorDelete(ctx context.Context, cmd *cli.Command) error {
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
		contributors, err := fetchContributors(ctx, client)
		if err != nil {
			return err
		}
		selected, err := menu.MultiSelect(w, contributors, "contributor",
			func(c contributorOption) string {
				if c.Identifier != "" {
					return c.Identifier
				}
				return extractRkey(c.URI)
			},
			func(c contributorOption) string { return c.DisplayName },
		)
		if err != nil {
			return err
		}
		if !menu.ConfirmBulkDelete(w, os.Stdin, len(selected), "contributor") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		for _, c := range selected {
			aturi, _ := syntax.ParseATURI(c.URI)
			if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
				fmt.Fprintf(w, "  Warning: %v\n", err)
			} else {
				fmt.Fprintf(w, "Deleted contributor: %s\n", extractRkey(c.URI))
			}
		}
		return nil
	}

	uri := resolveRecordURI(did, atproto.CollectionContributorInfo, id)
	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, fmt.Sprintf("Delete contributor %s?", extractRkey(uri))) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
		return fmt.Errorf("failed to delete contributor: %w", err)
	}
	fmt.Fprintf(w, "Deleted contributor: %s\n", extractRkey(uri))
	return nil
}

func runContributorGet(ctx context.Context, cmd *cli.Command) error {
	return runSimpleGet(ctx, cmd, atproto.CollectionContributorInfo, "contributor")
}

// selectContributor shows a menu to select an existing contributor or create a new one.
// Used by activity create for linking contributors.
func selectContributor(ctx context.Context, client *atclient.APIClient, w io.Writer) (*contributorOption, error) {
	contributors, err := fetchContributors(ctx, client)
	if err != nil {
		return nil, err
	}

	selected, isCreate, err := menu.SingleSelectWithCreate(w, contributors, "contributor",
		func(c contributorOption) string {
			if c.DisplayName != "" {
				return c.DisplayName
			}
			return c.Identifier
		},
		func(c contributorOption) string {
			if c.DisplayName != "" {
				return c.Identifier
			}
			return ""
		},
		"Create new contributor...",
	)
	if err != nil {
		return nil, err
	}

	if isCreate {
		return createContributorInline(ctx, client, w)
	}
	return selected, nil
}

func createContributorInline(ctx context.Context, client *atclient.APIClient, w io.Writer) (*contributorOption, error) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  \033[1mNew Contributor\033[0m")

	identifier, err := prompt.ReadLineWithDefault(w, os.Stdin, "  Identifier", "DID or profile URI", "")
	if err != nil {
		return nil, err
	}
	if identifier == "" {
		return nil, fmt.Errorf("identifier is required")
	}

	displayName, err := prompt.ReadOptionalField(w, os.Stdin, "  Display name", "max 100 chars, optional")
	if err != nil {
		return nil, err
	}

	record := map[string]any{
		"$type":     atproto.CollectionContributorInfo,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}
	if identifier != "" {
		record["identifier"] = identifier
	}
	if displayName != "" {
		record["displayName"] = displayName
	}

	uri, cid, err := atproto.CreateRecord(ctx, client, atproto.CollectionContributorInfo, record)
	if err != nil {
		return nil, fmt.Errorf("failed to create contributor: %w", err)
	}

	fmt.Fprintf(w, "  \033[32m\u2713\033[0m Created contributor: %s\n", uri)
	return &contributorOption{
		URI:         uri,
		CID:         cid,
		Identifier:  identifier,
		DisplayName: displayName,
	}, nil
}
