package cmd

import (
	"context"
	"errors"
	"fmt"
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

type contributionOption struct {
	URI         string
	CID         string
	Rkey        string
	Role        string
	Description string
	StartDate   string
	EndDate     string
	Created     string
}

func fetchContributions(ctx context.Context, client *atclient.APIClient, did string) ([]contributionOption, error) {
	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionContribution)
	if err != nil {
		return nil, fmt.Errorf("failed to list contributions: %w", err)
	}
	var result []contributionOption
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
		startDate := ""
		if sd := mapStr(e.Value, "startDate"); sd != "" {
			if t, err := time.Parse(time.RFC3339, sd); err == nil {
				startDate = t.Format("2006-01-02")
			}
		}
		endDate := ""
		if ed := mapStr(e.Value, "endDate"); ed != "" {
			if t, err := time.Parse(time.RFC3339, ed); err == nil {
				endDate = t.Format("2006-01-02")
			}
		}
		result = append(result, contributionOption{
			URI:         e.URI,
			CID:         e.CID,
			Rkey:        string(aturi.RecordKey()),
			Role:        mapStr(e.Value, "role"),
			Description: mapStr(e.Value, "contributionDescription"),
			StartDate:   startDate,
			EndDate:     endDate,
			Created:     created,
		})
	}
	return result, nil
}

func runContributionCreate(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer

	record := map[string]any{
		"$type":     atproto.CollectionContribution,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	role := cmd.String("role")
	description := cmd.String("description")
	startDate := cmd.String("start-date")
	endDate := cmd.String("end-date")

	if role == "" && description == "" && startDate == "" && endDate == "" {
		// Interactive mode: show all fields at once using huh form
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Role").
					Description("max 100 chars (optional)").
					CharLimit(100).
					Value(&role),

				huh.NewInput().
					Title("Contribution description").
					Description("max 10000 chars (optional)").
					CharLimit(10000).
					Value(&description),

				huh.NewInput().
					Title("Start date").
					Description("YYYY-MM-DD (optional)").
					Placeholder("2024-01-01").
					Value(&startDate),

				huh.NewInput().
					Title("End date").
					Description("YYYY-MM-DD (optional)").
					Placeholder("2024-12-31").
					Value(&endDate),
			).Title("Contribution Details"),
		).WithTheme(style.Theme())

		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return fmt.Errorf("cancelled")
			}
			return err
		}
	}

	// Add optional fields if provided
	if role != "" {
		record["role"] = role
	}
	if description != "" {
		record["contributionDescription"] = description
	}
	if startDate != "" {
		record["startDate"] = normalizeDate(startDate)
	}
	if endDate != "" {
		record["endDate"] = normalizeDate(endDate)
	}

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionContribution, record)
	if err != nil {
		return fmt.Errorf("failed to create contribution: %w", err)
	}

	fmt.Fprintf(w, "\n\033[32m✓\033[0m Created contribution: %s\n", uri)
	return nil
}

func runContributionEdit(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	arg := cmd.Args().First()
	var uri string
	if arg == "" {
		contributions, err := fetchContributions(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.SingleSelect(w, contributions, "contribution",
			func(c contributionOption) string {
				if c.Role != "" {
					return c.Role
				}
				return c.Rkey
			},
			func(c contributionOption) string {
				if c.Description != "" {
					if len(c.Description) > 50 {
						return c.Description[:47] + "..."
					}
					return c.Description
				}
				return ""
			},
		)
		if err != nil {
			return err
		}
		uri = selected.URI
	} else {
		uri = resolveRecordURI(did, atproto.CollectionContribution, arg)
	}

	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	existing, cid, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("contribution not found: %s", extractRkey(uri))
	}

	// Get current values
	currentRole := mapStr(existing, "role")
	currentDesc := mapStr(existing, "contributionDescription")
	currentStartDate := mapStr(existing, "startDate")
	currentEndDate := mapStr(existing, "endDate")

	// Get new values from flags or prompts
	newRole := cmd.String("role")
	newDesc := cmd.String("description")
	newStartDate := cmd.String("start-date")
	newEndDate := cmd.String("end-date")

	// Apply defaults and track changes
	changed := false
	isInteractive := newRole == "" && newDesc == "" && newStartDate == "" && newEndDate == ""

	if isInteractive {
		// Interactive mode
		newRole = currentRole
		newDesc = currentDesc
		newStartDate = currentStartDate
		newEndDate = currentEndDate

		// Format dates for display
		if newStartDate != "" {
			if t, err := time.Parse(time.RFC3339, newStartDate); err == nil {
				newStartDate = t.Format("2006-01-02")
			}
		}
		if newEndDate != "" {
			if t, err := time.Parse(time.RFC3339, newEndDate); err == nil {
				newEndDate = t.Format("2006-01-02")
			}
		}

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Role").
					Description("Max 100 chars").
					CharLimit(100).
					Value(&newRole),

				huh.NewInput().
					Title("Contribution description").
					Description("Max 10000 chars").
					CharLimit(10000).
					Value(&newDesc),

				huh.NewInput().
					Title("Start date").
					Description("YYYY-MM-DD").
					Placeholder("2024-01-01").
					Value(&newStartDate),

				huh.NewInput().
					Title("End date").
					Description("YYYY-MM-DD").
					Placeholder("2024-12-31").
					Value(&newEndDate),
			).Title("Edit Contribution"),
		).WithTheme(style.Theme())

		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return fmt.Errorf("cancelled")
			}
			return err
		}
	}

	// Normalize dates
	if newStartDate != "" {
		newStartDate = normalizeDate(newStartDate)
	}
	if newEndDate != "" {
		newEndDate = normalizeDate(newEndDate)
	}

	// Update fields if changed
	if newRole != currentRole {
		if newRole == "" {
			delete(existing, "role")
		} else {
			existing["role"] = newRole
		}
		changed = true
	}
	if newDesc != currentDesc {
		if newDesc == "" {
			delete(existing, "contributionDescription")
		} else {
			existing["contributionDescription"] = newDesc
		}
		changed = true
	}
	if newStartDate != currentStartDate {
		if newStartDate == "" {
			delete(existing, "startDate")
		} else {
			existing["startDate"] = newStartDate
		}
		changed = true
	}
	if newEndDate != currentEndDate {
		if newEndDate == "" {
			delete(existing, "endDate")
		} else {
			existing["endDate"] = newEndDate
		}
		changed = true
	}

	if !changed {
		fmt.Fprintln(w, "No changes.")
		return nil
	}

	resultURI, err := atproto.PutRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String(), existing, &cid)
	if err != nil {
		return fmt.Errorf("failed to update contribution: %w", err)
	}

	fmt.Fprintf(w, "\033[32m✓\033[0m Updated contribution: %s\n", resultURI)
	return nil
}

func runContributionDelete(ctx context.Context, cmd *cli.Command) error {
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
		contributions, err := fetchContributions(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.MultiSelect(w, contributions, "contribution",
			func(c contributionOption) string {
				if c.Role != "" {
					return c.Role
				}
				return c.Rkey
			},
			func(c contributionOption) string {
				if c.Description != "" {
					if len(c.Description) > 50 {
						return c.Description[:47] + "..."
					}
					return c.Description
				}
				return ""
			},
		)
		if err != nil {
			return err
		}
		if !menu.ConfirmBulkDelete(w, os.Stdin, len(selected), "contribution") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		for _, c := range selected {
			aturi, _ := syntax.ParseATURI(c.URI)
			if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
				fmt.Fprintf(w, "  Warning: %v\n", err)
			} else {
				fmt.Fprintf(w, "Deleted contribution: %s\n", c.Rkey)
			}
		}
		return nil
	}

	uri := resolveRecordURI(did, atproto.CollectionContribution, id)
	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, fmt.Sprintf("Delete contribution %s?", extractRkey(uri))) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
		return fmt.Errorf("failed to delete contribution: %w", err)
	}
	fmt.Fprintf(w, "Deleted contribution: %s\n", extractRkey(uri))
	return nil
}

func runContributionList(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	did := client.AccountDID.String()

	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionContribution)
	if err != nil {
		return fmt.Errorf("failed to list contributions: %w", err)
	}

	if cmd.Bool("json") {
		var records []map[string]any
		for _, e := range entries {
			records = append(records, map[string]any{"uri": e.URI, "record": e.Value})
		}
		fmt.Fprintln(w, prettyJSON(records))
		return nil
	}

	fmt.Fprintf(w, "\033[1m%-15s %-20s %-35s %-12s %-12s %s\033[0m\n", "ID", "ROLE", "DESCRIPTION", "START", "END", "CREATED")
	fmt.Fprintf(w, "%-15s %-20s %-35s %-12s %-12s %s\n",
		strings.Repeat("-", 13), strings.Repeat("-", 18),
		strings.Repeat("-", 33), strings.Repeat("-", 10),
		strings.Repeat("-", 10), strings.Repeat("-", 10))

	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		id := string(aturi.RecordKey())
		role := mapStr(e.Value, "role")
		description := mapStr(e.Value, "contributionDescription")

		if len(role) > 18 {
			role = role[:15] + "..."
		}
		if len(description) > 33 {
			description = description[:30] + "..."
		}

		startDate := "-"
		if sd := mapStr(e.Value, "startDate"); sd != "" {
			if t, err := time.Parse(time.RFC3339, sd); err == nil {
				startDate = t.Format("2006-01-02")
			}
		}

		endDate := "-"
		if ed := mapStr(e.Value, "endDate"); ed != "" {
			if t, err := time.Parse(time.RFC3339, ed); err == nil {
				endDate = t.Format("2006-01-02")
			}
		}

		created := "-"
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		fmt.Fprintf(w, "%-15s %-20s %-35s %-12s %-12s %s\n", id, role, description, startDate, endDate, created)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "\033[90m(no contributions found)\033[0m")
	}
	return nil
}

func runContributionGet(ctx context.Context, cmd *cli.Command) error {
	return runSimpleGet(ctx, cmd, atproto.CollectionContribution, "contribution")
}
