package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v3"

	"github.com/GainForest/hypercerts-cli/internal/atproto"
	"github.com/GainForest/hypercerts-cli/internal/menu"
	"github.com/GainForest/hypercerts-cli/internal/style"
)

// runProfileSet creates or updates the user's profile record (rkey="self").
func runProfileSet(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	displayName := cmd.String("display-name")
	description := cmd.String("description")
	pronouns := cmd.String("pronouns")
	website := cmd.String("website")

	// If no flags provided, use interactive form
	if displayName == "" && description == "" && pronouns == "" && website == "" {
		// Try to load existing record to pre-populate form
		existing, _, err := atproto.GetRecord(ctx, client, did, atproto.CollectionActorProfile, "self")
		if err == nil {
			displayName = mapStr(existing, "displayName")
			description = mapStr(existing, "description")
			pronouns = mapStr(existing, "pronouns")
			website = mapStr(existing, "website")
		}

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Display name").
					Description("Max 64 graphemes").
					CharLimit(640).
					Value(&displayName),

				huh.NewText().
					Title("Description").
					Description("Max 256 graphemes").
					CharLimit(2560).
					Value(&description),

				huh.NewInput().
					Title("Pronouns").
					Description("Max 20 graphemes").
					CharLimit(200).
					Value(&pronouns),

				huh.NewInput().
					Title("Website").
					Description("URI format").
					Value(&website),
			).Title("Profile"),
		).WithTheme(style.Theme())

		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return fmt.Errorf("cancelled")
			}
			return err
		}
	}

	// Build record
	record := map[string]any{
		"$type":     atproto.CollectionActorProfile,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}
	if displayName != "" {
		record["displayName"] = displayName
	}
	if description != "" {
		record["description"] = description
	}
	if pronouns != "" {
		record["pronouns"] = pronouns
	}
	if website != "" {
		record["website"] = website
	}

	// Check if record exists
	_, cid, err := atproto.GetRecord(ctx, client, did, atproto.CollectionActorProfile, "self")
	if err == nil {
		// Record exists, use PutRecord
		uri, err := atproto.PutRecord(ctx, client, did, atproto.CollectionActorProfile, "self", record, &cid)
		if err != nil {
			return fmt.Errorf("failed to update profile: %w", err)
		}
		fmt.Fprintf(w, "\033[32m✓\033[0m Updated profile: %s\n", uri)
	} else {
		// Record doesn't exist, create with rkey="self"
		uri, _, err := atproto.CreateRecordWithRkey(ctx, client, atproto.CollectionActorProfile, "self", record)
		if err != nil {
			return fmt.Errorf("failed to create profile: %w", err)
		}
		fmt.Fprintf(w, "\033[32m✓\033[0m Created profile: %s\n", uri)
	}

	return nil
}

// runProfileGet fetches and displays the user's profile record.
func runProfileGet(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	record, _, err := atproto.GetRecord(ctx, client, did, atproto.CollectionActorProfile, "self")
	if err != nil {
		return fmt.Errorf("profile not found: %w", err)
	}

	if cmd.Bool("json") {
		fmt.Fprintln(w, prettyJSON(record))
		return nil
	}

	// Display formatted profile
	fmt.Fprintf(w, "\033[1mProfile\033[0m\n")
	fmt.Fprintf(w, "URI: at://%s/%s/self\n\n", did, atproto.CollectionActorProfile)

	if displayName := mapStr(record, "displayName"); displayName != "" {
		fmt.Fprintf(w, "\033[1mDisplay name:\033[0m %s\n", displayName)
	}
	if description := mapStr(record, "description"); description != "" {
		fmt.Fprintf(w, "\033[1mDescription:\033[0m\n%s\n\n", description)
	}
	if pronouns := mapStr(record, "pronouns"); pronouns != "" {
		fmt.Fprintf(w, "\033[1mPronouns:\033[0m %s\n", pronouns)
	}
	if website := mapStr(record, "website"); website != "" {
		fmt.Fprintf(w, "\033[1mWebsite:\033[0m %s\n", website)
	}
	if createdAt := mapStr(record, "createdAt"); createdAt != "" {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			fmt.Fprintf(w, "\033[1mCreated:\033[0m %s\n", t.Format("2006-01-02 15:04:05"))
		}
	}

	return nil
}

// runProfileDelete deletes the user's profile record.
func runProfileDelete(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	// Check if record exists
	_, _, err = atproto.GetRecord(ctx, client, did, atproto.CollectionActorProfile, "self")
	if err != nil {
		return fmt.Errorf("profile not found")
	}

	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, "Delete your profile?") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}

	if err := atproto.DeleteRecord(ctx, client, did, atproto.CollectionActorProfile, "self"); err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}

	fmt.Fprintf(w, "Deleted profile\n")
	return nil
}

// runOrganizationSet creates or updates the user's organization record (rkey="self").
func runOrganizationSet(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	orgType := cmd.String("type")
	foundedDate := cmd.String("founded-date")
	url := cmd.String("url")
	urlLabel := cmd.String("url-label")

	// If no flags provided, use interactive form
	if orgType == "" && foundedDate == "" && url == "" {
		// Try to load existing record to pre-populate form
		existing, _, err := atproto.GetRecord(ctx, client, did, atproto.CollectionActorOrganization, "self")
		if err == nil {
			// Extract existing values
			if types, ok := existing["organizationType"].([]any); ok && len(types) > 0 {
				if s, ok := types[0].(string); ok {
					orgType = s
				}
			}
			foundedDate = mapStr(existing, "foundedDate")
			if urls, ok := existing["urls"].([]any); ok && len(urls) > 0 {
				if urlMap, ok := urls[0].(map[string]any); ok {
					url = mapStr(urlMap, "url")
					urlLabel = mapStr(urlMap, "label")
				}
			}
		}

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Organization type").
					Description("e.g., nonprofit, ngo, company").
					Value(&orgType),

				huh.NewInput().
					Title("Founded date").
					Description("YYYY-MM-DD format").
					Value(&foundedDate),

				huh.NewInput().
					Title("URL").
					Description("Organization website").
					Value(&url),

				huh.NewInput().
					Title("URL label").
					Description("Label for the URL (e.g., Website)").
					Value(&urlLabel),
			).Title("Organization"),
		).WithTheme(style.Theme())

		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return fmt.Errorf("cancelled")
			}
			return err
		}
	}

	// Build record
	record := map[string]any{
		"$type":     atproto.CollectionActorOrganization,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	if orgType != "" {
		// Split comma-separated types
		types := strings.Split(orgType, ",")
		var typeArray []any
		for _, t := range types {
			t = strings.TrimSpace(t)
			if t != "" {
				typeArray = append(typeArray, t)
			}
		}
		if len(typeArray) > 0 {
			record["organizationType"] = typeArray
		}
	}

	if foundedDate != "" {
		normalized := normalizeDate(foundedDate)
		if normalized != "" {
			record["foundedDate"] = normalized
		}
	}

	if url != "" {
		urlEntry := map[string]any{"url": url}
		if urlLabel != "" {
			urlEntry["label"] = urlLabel
		}
		record["urls"] = []any{urlEntry}
	}

	// Check if record exists
	_, cid, err := atproto.GetRecord(ctx, client, did, atproto.CollectionActorOrganization, "self")
	if err == nil {
		// Record exists, use PutRecord
		uri, err := atproto.PutRecord(ctx, client, did, atproto.CollectionActorOrganization, "self", record, &cid)
		if err != nil {
			return fmt.Errorf("failed to update organization: %w", err)
		}
		fmt.Fprintf(w, "\033[32m✓\033[0m Updated organization: %s\n", uri)
	} else {
		// Record doesn't exist, create with rkey="self"
		uri, _, err := atproto.CreateRecordWithRkey(ctx, client, atproto.CollectionActorOrganization, "self", record)
		if err != nil {
			return fmt.Errorf("failed to create organization: %w", err)
		}
		fmt.Fprintf(w, "\033[32m✓\033[0m Created organization: %s\n", uri)
	}

	return nil
}

// runOrganizationGet fetches and displays the user's organization record.
func runOrganizationGet(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	record, _, err := atproto.GetRecord(ctx, client, did, atproto.CollectionActorOrganization, "self")
	if err != nil {
		return fmt.Errorf("organization not found: %w", err)
	}

	if cmd.Bool("json") {
		fmt.Fprintln(w, prettyJSON(record))
		return nil
	}

	// Display formatted organization
	fmt.Fprintf(w, "\033[1mOrganization\033[0m\n")
	fmt.Fprintf(w, "URI: at://%s/%s/self\n\n", did, atproto.CollectionActorOrganization)

	if types, ok := record["organizationType"].([]any); ok && len(types) > 0 {
		var typeStrs []string
		for _, t := range types {
			if s, ok := t.(string); ok {
				typeStrs = append(typeStrs, s)
			}
		}
		if len(typeStrs) > 0 {
			fmt.Fprintf(w, "\033[1mType:\033[0m %s\n", strings.Join(typeStrs, ", "))
		}
	}

	if foundedDate := mapStr(record, "foundedDate"); foundedDate != "" {
		if t, err := time.Parse(time.RFC3339, foundedDate); err == nil {
			fmt.Fprintf(w, "\033[1mFounded:\033[0m %s\n", t.Format("2006-01-02"))
		}
	}

	if urls, ok := record["urls"].([]any); ok && len(urls) > 0 {
		fmt.Fprintf(w, "\033[1mURLs:\033[0m\n")
		for _, u := range urls {
			if urlMap, ok := u.(map[string]any); ok {
				url := mapStr(urlMap, "url")
				label := mapStr(urlMap, "label")
				if label != "" {
					fmt.Fprintf(w, "  %s: %s\n", label, url)
				} else {
					fmt.Fprintf(w, "  %s\n", url)
				}
			}
		}
	}

	if createdAt := mapStr(record, "createdAt"); createdAt != "" {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			fmt.Fprintf(w, "\033[1mCreated:\033[0m %s\n", t.Format("2006-01-02 15:04:05"))
		}
	}

	return nil
}

// runOrganizationDelete deletes the user's organization record.
func runOrganizationDelete(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	// Check if record exists
	_, _, err = atproto.GetRecord(ctx, client, did, atproto.CollectionActorOrganization, "self")
	if err != nil {
		return fmt.Errorf("organization not found")
	}

	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, "Delete your organization?") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}

	if err := atproto.DeleteRecord(ctx, client, did, atproto.CollectionActorOrganization, "self"); err != nil {
		return fmt.Errorf("failed to delete organization: %w", err)
	}

	fmt.Fprintf(w, "Deleted organization\n")
	return nil
}
