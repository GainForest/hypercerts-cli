package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/urfave/cli/v3"

	"github.com/GainForest/hypercerts-cli/internal/atproto"
	"github.com/GainForest/hypercerts-cli/internal/github"
)

type createdContributor struct {
	uri           string
	cid           string
	contributions int
}

func runFromGitHub(ctx context.Context, cmd *cli.Command, client *atclient.APIClient, repoInput string) error {
	w := cmd.Root().Writer
	token := cmd.String("github-token")

	// Parse repo input
	owner, repo, err := github.ParseRepo(repoInput)
	if err != nil {
		return fmt.Errorf("invalid repo format: %w", err)
	}

	// Fetch repo metadata
	repoInfo, err := github.FetchRepo(ctx, owner, repo, token)
	if err != nil {
		return fmt.Errorf("failed to fetch repo: %w", err)
	}

	// Fetch contributors
	contributors, err := github.FetchContributors(ctx, owner, repo, token)
	if err != nil {
		return fmt.Errorf("failed to fetch contributors: %w", err)
	}

	// Print summary
	fmt.Fprintf(w, "Importing from GitHub: %s\n", repoInfo.FullName)
	fmt.Fprintf(w, "  Description: %s\n", repoInfo.Description)
	fmt.Fprintf(w, "  Language: %s\n", repoInfo.Language)
	fmt.Fprintf(w, "  License: %s\n", repoInfo.License)
	if t, err := time.Parse(time.RFC3339, repoInfo.CreatedAt); err == nil {
		fmt.Fprintf(w, "  Created: %s\n", t.Format("2006-01-02"))
	}
	fmt.Fprintf(w, "  Contributors: %d\n", len(contributors))

	// Fetch existing contributors to avoid duplicates
	existingContribs, err := fetchContributors(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to fetch existing contributors: %w", err)
	}

	// Build a map of identifier -> (URI, CID) for quick lookup
	existingMap := make(map[string]struct {
		uri string
		cid string
	})
	for _, ec := range existingContribs {
		existingMap[ec.Identifier] = struct {
			uri string
			cid string
		}{uri: ec.URI, cid: ec.CID}
	}

	// Create contributor records (or reuse existing ones)
	var createdContribs []createdContributor
	for _, c := range contributors {
		// Check if contributor already exists
		if existing, found := existingMap[c.HTMLURL]; found {
			fmt.Fprintf(w, "  ✓ Found existing contributor: %s (%d commits)\n", c.Login, c.Contributions)
			createdContribs = append(createdContribs, createdContributor{
				uri:           existing.uri,
				cid:           existing.cid,
				contributions: c.Contributions,
			})
			continue
		}

		// Create new contributor record
		contribRecord := map[string]any{
			"$type":       atproto.CollectionContributorInfo,
			"createdAt":   time.Now().UTC().Format(time.RFC3339),
			"identifier":  c.HTMLURL,
			"displayName": c.Login,
		}
		if c.AvatarURL != "" {
			contribRecord["image"] = map[string]any{
				"$type": "org.hypercerts.defs#uri",
				"uri":   c.AvatarURL,
			}
		}

		uri, cid, err := atproto.CreateRecord(ctx, client, atproto.CollectionContributorInfo, contribRecord)
		if err != nil {
			return fmt.Errorf("failed to create contributor %s: %w", c.Login, err)
		}

		fmt.Fprintf(w, "  ✓ Created contributor: %s (%d commits)\n", c.Login, c.Contributions)
		createdContribs = append(createdContribs, createdContributor{
			uri:           uri,
			cid:           cid,
			contributions: c.Contributions,
		})
	}

	// Build activity record
	activityRecord := buildActivityFromGitHub(repoInfo, createdContribs)

	// Create activity record
	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionActivity, activityRecord)
	if err != nil {
		return fmt.Errorf("failed to create activity: %w", err)
	}

	fmt.Fprintf(w, "\033[32m✓\033[0m Created activity: %s\n", uri)
	return nil
}

func buildActivityFromGitHub(repo *github.RepoInfo, contribs []createdContributor) map[string]any {
	record := map[string]any{
		"$type":     atproto.CollectionActivity,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
		"title":     repo.FullName,
	}

	// Short description (truncate to 300 chars if needed)
	shortDesc := repo.Description
	if shortDesc == "" {
		shortDesc = fmt.Sprintf("%s — a %s project", repo.FullName, repo.Language)
	}
	if len(shortDesc) > 300 {
		shortDesc = shortDesc[:297] + "..."
	}
	record["shortDescription"] = shortDesc

	// Long description with GitHub URL and metadata
	var descParts []string
	descParts = append(descParts, fmt.Sprintf("GitHub: %s", repo.HTMLURL))
	if repo.License != "" {
		descParts = append(descParts, fmt.Sprintf("License: %s", repo.License))
	}
	if repo.Language != "" {
		descParts = append(descParts, fmt.Sprintf("Language: %s", repo.Language))
	}
	record["description"] = strings.Join(descParts, "\n")

	// Dates
	record["startDate"] = repo.CreatedAt
	record["endDate"] = time.Now().UTC().Format(time.RFC3339)

	// Image (owner avatar)
	if repo.AvatarURL != "" {
		record["image"] = map[string]any{
			"$type": "org.hypercerts.defs#uri",
			"uri":   repo.AvatarURL,
		}
	}

	// Contributors with proportional weights (raw commit counts)
	var contributorsArray []any
	for _, c := range contribs {
		obj := map[string]any{
			"contributorIdentity": buildStrongRef(c.uri, c.cid),
			"contributionWeight":  fmt.Sprintf("%d", c.contributions),
		}
		contributorsArray = append(contributorsArray, obj)
	}
	if len(contributorsArray) > 0 {
		record["contributors"] = contributorsArray
	}

	return record
}
