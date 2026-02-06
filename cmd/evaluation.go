package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/urfave/cli/v3"

	"github.com/GainForest/hypercerts-cli/internal/atproto"
	"github.com/GainForest/hypercerts-cli/internal/menu"
	"github.com/GainForest/hypercerts-cli/internal/prompt"
)

type evaluationOption struct {
	URI          string
	CID          string
	Rkey         string
	Summary      string
	SubjectRkey  string
	EvaluatorCnt int
	HasScore     bool
	ScoreValue   int
	ScoreMax     int
	Created      string
}

func fetchEvaluations(ctx context.Context, client *atclient.APIClient, did string) ([]evaluationOption, error) {
	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionEvaluation)
	if err != nil {
		return nil, fmt.Errorf("failed to list evaluations: %w", err)
	}
	var result []evaluationOption
	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}

		subjectRkey := ""
		if subject := mapMap(e.Value, "subject"); subject != nil {
			subjectRkey = extractRkey(mapStr(subject, "uri"))
		}

		evaluatorCnt := 0
		if evaluators := mapSlice(e.Value, "evaluators"); evaluators != nil {
			evaluatorCnt = len(evaluators)
		}

		hasScore := false
		scoreValue := 0
		scoreMax := 0
		if score := mapMap(e.Value, "score"); score != nil {
			hasScore = true
			if v, ok := score["value"].(float64); ok {
				scoreValue = int(v)
			}
			if m, ok := score["max"].(float64); ok {
				scoreMax = int(m)
			}
		}

		created := ""
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		summary := mapStr(e.Value, "summary")
		if len(summary) > 50 {
			summary = summary[:47] + "..."
		}

		result = append(result, evaluationOption{
			URI:          e.URI,
			CID:          e.CID,
			Rkey:         string(aturi.RecordKey()),
			Summary:      summary,
			SubjectRkey:  subjectRkey,
			EvaluatorCnt: evaluatorCnt,
			HasScore:     hasScore,
			ScoreValue:   scoreValue,
			ScoreMax:     scoreMax,
			Created:      created,
		})
	}
	return result, nil
}

// promptEvaluators prompts for evaluator DIDs.
func promptEvaluators(w io.Writer) ([]string, error) {
	var evaluators []string
	for {
		var did string
		var err error
		if len(evaluators) == 0 {
			did, err = prompt.ReadRequired(w, os.Stdin, "Evaluator DID", "e.g. did:plc:abc123")
		} else {
			did, err = prompt.ReadOptionalField(w, os.Stdin, "Evaluator DID", "enter to finish")
		}
		if err != nil {
			return nil, err
		}
		if did == "" {
			break
		}
		evaluators = append(evaluators, did)

		if !menu.Confirm(w, os.Stdin, "Add another evaluator?") {
			break
		}
	}
	return evaluators, nil
}

// promptContentURIsForEval prompts for content URIs (reports, methodology docs).
func promptContentURIsForEval(w io.Writer) ([]map[string]any, error) {
	var content []map[string]any
	fmt.Fprintln(w)
	if !menu.Confirm(w, os.Stdin, "Add content URIs (reports, methodology)?") {
		return nil, nil
	}
	for {
		uri, err := prompt.ReadLineWithDefault(w, os.Stdin, "Content URI", "URL to report/methodology", "")
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

// promptScore prompts for score (min, max, value).
func promptScore(w io.Writer) (map[string]any, error) {
	fmt.Fprintln(w)
	if !menu.Confirm(w, os.Stdin, "Add score?") {
		return nil, nil
	}

	minStr, err := prompt.ReadLineWithDefault(w, os.Stdin, "Score min", "e.g. 0 or 1", "0")
	if err != nil {
		return nil, err
	}
	min, err := strconv.Atoi(minStr)
	if err != nil {
		return nil, fmt.Errorf("invalid min score: must be an integer")
	}

	maxStr, err := prompt.ReadLineWithDefault(w, os.Stdin, "Score max", "e.g. 5 or 10", "10")
	if err != nil {
		return nil, err
	}
	max, err := strconv.Atoi(maxStr)
	if err != nil {
		return nil, fmt.Errorf("invalid max score: must be an integer")
	}

	valueStr, err := prompt.ReadLineWithDefault(w, os.Stdin, "Score value", fmt.Sprintf("between %d and %d", min, max), "")
	if err != nil {
		return nil, err
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return nil, fmt.Errorf("invalid score value: must be an integer")
	}
	if value < min || value > max {
		return nil, fmt.Errorf("score value must be between %d and %d", min, max)
	}

	return map[string]any{
		"min":   min,
		"max":   max,
		"value": value,
	}, nil
}

// selectMeasurements allows selecting multiple measurements to link to evaluation.
func selectMeasurements(ctx context.Context, client *atclient.APIClient, w io.Writer) ([]map[string]any, error) {
	did := client.AccountDID.String()
	measurements, err := fetchMeasurements(ctx, client, did)
	if err != nil {
		return nil, err
	}

	if len(measurements) == 0 {
		fmt.Fprintln(w, "\033[90m(no measurements available)\033[0m")
		return nil, nil
	}

	selected, err := menu.MultiSelect(w, measurements, "measurement",
		func(m measurementOption) string {
			return fmt.Sprintf("%s: %s %s", m.Metric, m.Value, m.Unit)
		},
		func(m measurementOption) string {
			if m.SubjectRkey != "" {
				return "activity: " + m.SubjectRkey
			}
			return m.Rkey
		},
	)
	if err != nil {
		return nil, err
	}

	// Fetch CIDs for selected measurements
	var refs []map[string]any
	for _, m := range selected {
		aturi, _ := syntax.ParseATURI(m.URI)
		_, cid, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
		if err != nil {
			continue
		}
		refs = append(refs, buildStrongRef(m.URI, cid))
	}
	return refs, nil
}

func runEvaluationCreate(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer

	record := map[string]any{
		"$type":     atproto.CollectionEvaluation,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	// Subject (what is being evaluated) - optional but recommended
	fmt.Fprintln(w, "Select what to evaluate (activity, measurement, etc.):")
	subjectURI, subjectCID, err := selectActivity(ctx, client, w)
	if err != nil && err != menu.ErrCancelled {
		return err
	}
	if subjectURI != "" {
		record["subject"] = buildStrongRef(subjectURI, subjectCID)
	}

	// Evaluators - required
	fmt.Fprintln(w)
	evaluators, err := promptEvaluators(w)
	if err != nil {
		return err
	}
	record["evaluators"] = evaluators

	// Summary - required
	summary := cmd.String("summary")
	if summary == "" {
		fmt.Fprintln(w)
		summary, err = prompt.ReadRequired(w, os.Stdin, "Summary", "brief evaluation summary")
		if err != nil {
			return err
		}
	}
	record["summary"] = summary

	// Content URIs - optional
	content, err := promptContentURIsForEval(w)
	if err != nil {
		return err
	}
	if len(content) > 0 {
		record["content"] = content
	}

	// Score - optional
	score, err := promptScore(w)
	if err != nil {
		return err
	}
	if score != nil {
		record["score"] = score
	}

	// Measurements - optional
	fmt.Fprintln(w)
	if menu.Confirm(w, os.Stdin, "Link measurements to this evaluation?") {
		measurements, err := selectMeasurements(ctx, client, w)
		if err != nil {
			return err
		}
		if len(measurements) > 0 {
			record["measurements"] = measurements
		}
	}

	// Location - optional
	fmt.Fprintln(w)
	if menu.Confirm(w, os.Stdin, "Add location?") {
		loc, err := selectLocation(ctx, client, w)
		if err != nil {
			return err
		}
		record["location"] = buildStrongRef(loc.URI, loc.CID)
	}

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionEvaluation, record)
	if err != nil {
		return fmt.Errorf("failed to create evaluation: %w", err)
	}

	fmt.Fprintf(w, "\n\033[32m✓\033[0m Created evaluation: %s\n", uri)
	return nil
}

func runEvaluationEdit(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	arg := cmd.Args().First()
	var uri string
	if arg == "" {
		evaluations, err := fetchEvaluations(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.SingleSelect(w, evaluations, "evaluation",
			func(e evaluationOption) string { return e.Summary },
			func(e evaluationOption) string {
				if e.HasScore {
					return fmt.Sprintf("%d/%d", e.ScoreValue, e.ScoreMax)
				}
				return e.Rkey
			},
		)
		if err != nil {
			return err
		}
		uri = selected.URI
	} else {
		uri = resolveRecordURI(did, atproto.CollectionEvaluation, arg)
	}

	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	existing, cid, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("evaluation not found: %s", extractRkey(uri))
	}

	// Get current values
	currentSummary := mapStr(existing, "summary")

	changed := false
	isInteractive := cmd.String("summary") == ""

	if isInteractive {
		// Summary
		newSummary, err := prompt.ReadLineWithDefault(w, os.Stdin, "Summary", "required", currentSummary)
		if err != nil {
			return err
		}
		if newSummary != currentSummary {
			existing["summary"] = newSummary
			changed = true
		}

		// Score
		existingScore := mapMap(existing, "score")
		scoreLabel := "Add score?"
		if existingScore != nil {
			scoreLabel = "Replace score?"
		}
		fmt.Fprintln(w)
		if menu.Confirm(w, os.Stdin, scoreLabel) {
			score, err := promptScore(w)
			if err != nil {
				return err
			}
			if score != nil {
				existing["score"] = score
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

		// Measurements
		existingMeasurements := mapSlice(existing, "measurements")
		measLabel := "Add measurements?"
		if len(existingMeasurements) > 0 {
			measLabel = fmt.Sprintf("Replace %d measurement(s)?", len(existingMeasurements))
		}
		fmt.Fprintln(w)
		if menu.Confirm(w, os.Stdin, measLabel) {
			measurements, err := selectMeasurements(ctx, client, w)
			if err != nil {
				return err
			}
			if len(measurements) > 0 {
				existing["measurements"] = measurements
				changed = true
			}
		}
	} else {
		// Non-interactive mode
		newSummary := cmd.String("summary")
		if newSummary != "" && newSummary != currentSummary {
			existing["summary"] = newSummary
			changed = true
		}
	}

	if !changed {
		fmt.Fprintln(w, "No changes.")
		return nil
	}

	resultURI, err := atproto.PutRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String(), existing, &cid)
	if err != nil {
		return fmt.Errorf("failed to update evaluation: %w", err)
	}

	fmt.Fprintf(w, "\033[32m✓\033[0m Updated evaluation: %s\n", resultURI)
	return nil
}

func runEvaluationDelete(ctx context.Context, cmd *cli.Command) error {
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
		evaluations, err := fetchEvaluations(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.MultiSelect(w, evaluations, "evaluation",
			func(e evaluationOption) string { return e.Summary },
			func(e evaluationOption) string {
				if e.HasScore {
					return fmt.Sprintf("%d/%d", e.ScoreValue, e.ScoreMax)
				}
				return e.Rkey
			},
		)
		if err != nil {
			return err
		}
		if !menu.ConfirmBulkDelete(w, os.Stdin, len(selected), "evaluation") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		for _, e := range selected {
			aturi, _ := syntax.ParseATURI(e.URI)
			if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
				fmt.Fprintf(w, "  Warning: %v\n", err)
			} else {
				fmt.Fprintf(w, "Deleted evaluation: %s\n", e.Rkey)
			}
		}
		return nil
	}

	uri := resolveRecordURI(did, atproto.CollectionEvaluation, id)
	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, fmt.Sprintf("Delete evaluation %s?", extractRkey(uri))) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
		return fmt.Errorf("failed to delete evaluation: %w", err)
	}
	fmt.Fprintf(w, "Deleted evaluation: %s\n", extractRkey(uri))
	return nil
}

func runEvaluationList(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	did := client.AccountDID.String()

	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionEvaluation)
	if err != nil {
		return fmt.Errorf("failed to list evaluations: %w", err)
	}

	if cmd.Bool("json") {
		var records []map[string]any
		for _, e := range entries {
			records = append(records, map[string]any{"uri": e.URI, "record": e.Value})
		}
		fmt.Fprintln(w, prettyJSON(records))
		return nil
	}

	fmt.Fprintf(w, "\033[1m%-15s %-35s %-10s %-10s %-10s %s\033[0m\n", "ID", "SUMMARY", "SUBJECT", "EVALUATORS", "SCORE", "CREATED")
	fmt.Fprintf(w, "%-15s %-35s %-10s %-10s %-10s %s\n",
		strings.Repeat("-", 13), strings.Repeat("-", 33),
		strings.Repeat("-", 8), strings.Repeat("-", 8),
		strings.Repeat("-", 8), strings.Repeat("-", 10))

	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		id := string(aturi.RecordKey())

		summary := mapStr(e.Value, "summary")
		if len(summary) > 33 {
			summary = summary[:30] + "..."
		}

		subjectRkey := "-"
		if subject := mapMap(e.Value, "subject"); subject != nil {
			subjectRkey = extractRkey(mapStr(subject, "uri"))
			if len(subjectRkey) > 8 {
				subjectRkey = subjectRkey[:5] + "..."
			}
		}

		evaluatorCnt := 0
		if evaluators := mapSlice(e.Value, "evaluators"); evaluators != nil {
			evaluatorCnt = len(evaluators)
		}

		scoreStr := "-"
		if score := mapMap(e.Value, "score"); score != nil {
			if v, ok := score["value"].(float64); ok {
				if m, ok := score["max"].(float64); ok {
					scoreStr = fmt.Sprintf("%d/%d", int(v), int(m))
				}
			}
		}

		created := "-"
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		fmt.Fprintf(w, "%-15s %-35s %-10s %-10d %-10s %s\n", id, summary, subjectRkey, evaluatorCnt, scoreStr, created)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "\033[90m(no evaluations found)\033[0m")
	}
	return nil
}

func runEvaluationGet(ctx context.Context, cmd *cli.Command) error {
	return runSimpleGet(ctx, cmd, atproto.CollectionEvaluation, "evaluation")
}
