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

type measurementOption struct {
	URI         string
	Rkey        string
	Metric      string
	Unit        string
	Value       string
	SubjectURI  string
	SubjectRkey string
	Created     string
}

func fetchMeasurements(ctx context.Context, client *atclient.APIClient, did string) ([]measurementOption, error) {
	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionMeasurement)
	if err != nil {
		return nil, fmt.Errorf("failed to list measurements: %w", err)
	}
	var result []measurementOption
	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		subjectURI := ""
		subjectRkey := ""
		if subject := mapMap(e.Value, "subject"); subject != nil {
			subjectURI = mapStr(subject, "uri")
			subjectRkey = extractRkey(subjectURI)
		}
		created := ""
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}
		result = append(result, measurementOption{
			URI:         e.URI,
			Rkey:        string(aturi.RecordKey()),
			Metric:      mapStr(e.Value, "metric"),
			Unit:        mapStr(e.Value, "unit"),
			Value:       mapStr(e.Value, "value"),
			SubjectURI:  subjectURI,
			SubjectRkey: subjectRkey,
			Created:     created,
		})
	}
	return result, nil
}

func fetchMeasurementsForActivity(ctx context.Context, client *atclient.APIClient, did, activityURI string) ([]measurementOption, error) {
	all, err := fetchMeasurements(ctx, client, did)
	if err != nil {
		return nil, err
	}
	var result []measurementOption
	for _, m := range all {
		if m.SubjectURI == activityURI {
			result = append(result, m)
		}
	}
	return result, nil
}

// selectActivity shows a menu to select an activity for linking.
// Returns the URI and CID needed for building a strongRef.
func selectActivity(ctx context.Context, client *atclient.APIClient, w io.Writer) (uri, cid string, err error) {
	did := client.AccountDID.String()
	activities, err := fetchActivities(ctx, client, did)
	if err != nil {
		return "", "", err
	}

	if len(activities) == 0 {
		return "", "", fmt.Errorf("no activities found; create an activity first")
	}

	selected, err := menu.SingleSelect(w, activities, "activity",
		func(a activityOption) string { return a.Title },
		func(a activityOption) string { return a.Created },
	)
	if err != nil {
		return "", "", err
	}

	// Fetch the record to get CID for strongRef
	aturi, err := syntax.ParseATURI(selected.URI)
	if err != nil {
		return "", "", err
	}
	_, recordCID, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
	if err != nil {
		return "", "", fmt.Errorf("failed to get activity CID: %w", err)
	}

	return selected.URI, recordCID, nil
}

func runMeasurementCreate(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	record := map[string]any{
		"$type":     atproto.CollectionMeasurement,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	// Subject (activity link) - required
	activityFlag := cmd.String("activity")
	var subjectURI, subjectCID string
	if activityFlag != "" {
		subjectURI = resolveRecordURI(did, atproto.CollectionActivity, activityFlag)
		aturi, err := syntax.ParseATURI(subjectURI)
		if err != nil {
			return fmt.Errorf("invalid activity URI: %w", err)
		}
		_, subjectCID, err = atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
		if err != nil {
			return fmt.Errorf("activity not found: %s", activityFlag)
		}
	} else {
		fmt.Fprintln(w, "Select an activity to link this measurement to:")
		subjectURI, subjectCID, err = selectActivity(ctx, client, w)
		if err != nil {
			return err
		}
	}
	record["subject"] = buildStrongRef(subjectURI, subjectCID)

	// Check for non-interactive mode (flags provided)
	metric := cmd.String("metric")
	unit := cmd.String("unit")
	value := cmd.String("value")
	hasFlags := metric != "" || unit != "" || value != ""

	if s := cmd.String("start-date"); s != "" {
		record["startDate"] = normalizeDate(s)
		hasFlags = true
	}
	if s := cmd.String("end-date"); s != "" {
		record["endDate"] = normalizeDate(s)
		hasFlags = true
	}
	if s := cmd.String("method-type"); s != "" {
		record["methodType"] = s
		hasFlags = true
	}

	if hasFlags {
		// Non-interactive: require metric, unit, value via flags or prompt fallback
		if metric == "" {
			metric, err = prompt.ReadRequired(w, os.Stdin, "Metric", "e.g. 'trees planted'")
			if err != nil {
				return err
			}
		}
		if unit == "" {
			unit, err = prompt.ReadRequired(w, os.Stdin, "Unit", "e.g. 'count', 'kg', 'hectares'")
			if err != nil {
				return err
			}
		}
		if value == "" {
			value, err = prompt.ReadRequired(w, os.Stdin, "Value", "numeric")
			if err != nil {
				return err
			}
		}
		record["metric"] = metric
		record["unit"] = unit
		record["value"] = value
	} else {
		// Interactive: show all fields at once using huh form
		var startDate, endDate, methodType, methodURI, comment string
		var addLocations bool

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Metric").
					Description("What is being measured, e.g. 'trees planted'").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("metric is required")
						}
						return nil
					}).
					Value(&metric),

				huh.NewInput().
					Title("Unit").
					Description("Unit of measurement, e.g. 'count', 'kg', 'hectares'").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("unit is required")
						}
						return nil
					}).
					Value(&unit),

				huh.NewInput().
					Title("Value").
					Description("Numeric measurement value").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("value is required")
						}
						return nil
					}).
					Value(&value),
			).Title("Measurement Data"),

			huh.NewGroup(
				huh.NewInput().
					Title("Start date").
					Description("YYYY-MM-DD (optional)").
					Value(&startDate),

				huh.NewInput().
					Title("End date").
					Description("YYYY-MM-DD (optional)").
					Value(&endDate),

				huh.NewInput().
					Title("Method type").
					Description("Short methodology ID, max 30 chars (optional)").
					CharLimit(30).
					Value(&methodType),

				huh.NewInput().
					Title("Method URI").
					Description("URI to methodology docs (optional)").
					Value(&methodURI),

				huh.NewInput().
					Title("Comment").
					Description("Additional notes, max 300 chars (optional)").
					CharLimit(300).
					Value(&comment),
			).Title("Optional Fields"),

			huh.NewGroup(
				huh.NewConfirm().
					Title("Add locations?").
					Description("Link geographic location records").
					Value(&addLocations),
			).Title("Linked Records"),
		).WithTheme(huh.ThemeBase16())

		err := form.Run()
		if err != nil {
			return err
		}

		record["metric"] = metric
		record["unit"] = unit
		record["value"] = value

		if startDate != "" {
			record["startDate"] = normalizeDate(startDate)
		}
		if endDate != "" {
			record["endDate"] = normalizeDate(endDate)
		}
		if methodType != "" {
			record["methodType"] = methodType
		}
		if methodURI != "" {
			record["methodURI"] = methodURI
		}
		if comment != "" {
			record["comment"] = comment
		}

		// Handle linked records interactively (need API calls)
		if addLocations {
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
	}

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionMeasurement, record)
	if err != nil {
		return fmt.Errorf("failed to create measurement: %w", err)
	}

	fmt.Fprintf(w, "\n\033[32m✓\033[0m Created measurement: %s\n", uri)
	return nil
}

func runMeasurementEdit(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	arg := cmd.Args().First()
	var uri string
	if arg == "" {
		measurements, err := fetchMeasurements(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.SingleSelect(w, measurements, "measurement",
			func(m measurementOption) string {
				return fmt.Sprintf("%s: %s %s", m.Metric, m.Value, m.Unit)
			},
			func(m measurementOption) string {
				if m.SubjectRkey != "" {
					return "activity: " + m.SubjectRkey
				}
				return ""
			},
		)
		if err != nil {
			return err
		}
		uri = selected.URI
	} else {
		uri = resolveRecordURI(did, atproto.CollectionMeasurement, arg)
	}

	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	existing, cid, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("measurement not found: %s", extractRkey(uri))
	}

	// Get current values
	currentMetric := mapStr(existing, "metric")
	currentUnit := mapStr(existing, "unit")
	currentValue := mapStr(existing, "value")
	currentStartDate := mapStr(existing, "startDate")
	currentEndDate := mapStr(existing, "endDate")

	// Get new values from flags or prompts
	newMetric := cmd.String("metric")
	newUnit := cmd.String("unit")
	newValue := cmd.String("value")
	newStartDate := cmd.String("start-date")
	newEndDate := cmd.String("end-date")

	// Apply defaults and track changes
	changed := false
	isInteractive := newMetric == "" && newUnit == "" && newValue == "" && newStartDate == "" && newEndDate == ""

	if isInteractive {
		// Interactive mode
		newMetric, err = prompt.ReadLineWithDefault(w, os.Stdin, "Metric", "required", currentMetric)
		if err != nil {
			return err
		}
		newUnit, err = prompt.ReadLineWithDefault(w, os.Stdin, "Unit", "required", currentUnit)
		if err != nil {
			return err
		}
		newValue, err = prompt.ReadLineWithDefault(w, os.Stdin, "Value", "required", currentValue)
		if err != nil {
			return err
		}

		fmt.Fprintln(w)
		if menu.Confirm(w, os.Stdin, "Edit optional fields?") {
			// Dates
			newStartDate, err = prompt.ReadLineWithDefault(w, os.Stdin, "Start date", "YYYY-MM-DD", currentStartDate)
			if err != nil {
				return err
			}
			newEndDate, err = prompt.ReadLineWithDefault(w, os.Stdin, "End date", "YYYY-MM-DD", currentEndDate)
			if err != nil {
				return err
			}

			// Method type
			currentMethodType := mapStr(existing, "methodType")
			newMethodType, err := prompt.ReadLineWithDefault(w, os.Stdin, "Method type", "short methodology ID", currentMethodType)
			if err != nil {
				return err
			}
			if newMethodType != "" && newMethodType != currentMethodType {
				existing["methodType"] = newMethodType
				changed = true
			}

			// Method URI
			currentMethodURI := mapStr(existing, "methodURI")
			newMethodURI, err := prompt.ReadLineWithDefault(w, os.Stdin, "Method URI", "URL to methodology docs", currentMethodURI)
			if err != nil {
				return err
			}
			if newMethodURI != "" && newMethodURI != currentMethodURI {
				existing["methodURI"] = newMethodURI
				changed = true
			}

			// Comment
			currentComment := mapStr(existing, "comment")
			newComment, err := prompt.ReadLineWithDefault(w, os.Stdin, "Comment", "max 300 chars", currentComment)
			if err != nil {
				return err
			}
			if newComment != "" && newComment != currentComment {
				existing["comment"] = newComment
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
		}
	}
	if newMetric == "" {
		newMetric = currentMetric
	}
	if newUnit == "" {
		newUnit = currentUnit
	}
	if newValue == "" {
		newValue = currentValue
	}

	if newMetric != currentMetric {
		existing["metric"] = newMetric
		changed = true
	}
	if newUnit != currentUnit {
		existing["unit"] = newUnit
		changed = true
	}
	if newValue != currentValue {
		existing["value"] = newValue
		changed = true
	}
	if newStartDate != "" && newStartDate != currentStartDate {
		existing["startDate"] = normalizeDate(newStartDate)
		changed = true
	}
	if newEndDate != "" && newEndDate != currentEndDate {
		existing["endDate"] = normalizeDate(newEndDate)
		changed = true
	}

	if !changed {
		fmt.Fprintln(w, "No changes.")
		return nil
	}

	resultURI, err := atproto.PutRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String(), existing, &cid)
	if err != nil {
		return fmt.Errorf("failed to update measurement: %w", err)
	}

	fmt.Fprintf(w, "\033[32m✓\033[0m Updated measurement: %s\n", resultURI)
	return nil
}

func runMeasurementDelete(ctx context.Context, cmd *cli.Command) error {
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
		measurements, err := fetchMeasurements(ctx, client, did)
		if err != nil {
			return err
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
			return err
		}
		if !menu.ConfirmBulkDelete(w, os.Stdin, len(selected), "measurement") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		for _, m := range selected {
			aturi, _ := syntax.ParseATURI(m.URI)
			if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
				fmt.Fprintf(w, "  Warning: %v\n", err)
			} else {
				fmt.Fprintf(w, "Deleted measurement: %s\n", m.Rkey)
			}
		}
		return nil
	}

	uri := resolveRecordURI(did, atproto.CollectionMeasurement, id)
	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, fmt.Sprintf("Delete measurement %s?", extractRkey(uri))) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
		return fmt.Errorf("failed to delete measurement: %w", err)
	}
	fmt.Fprintf(w, "Deleted measurement: %s\n", extractRkey(uri))
	return nil
}

func runMeasurementList(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	did := client.AccountDID.String()

	// Filter by activity if specified
	activityFilter := cmd.String("activity")
	var entries []measurementOption
	if activityFilter != "" {
		activityURI := resolveRecordURI(did, atproto.CollectionActivity, activityFilter)
		entries, err = fetchMeasurementsForActivity(ctx, client, did, activityURI)
	} else {
		entries, err = fetchMeasurements(ctx, client, did)
	}
	if err != nil {
		return err
	}

	if cmd.Bool("json") {
		// Re-fetch full records for JSON output
		rawEntries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionMeasurement)
		if err != nil {
			return fmt.Errorf("failed to list measurements: %w", err)
		}
		var records []map[string]any
		for _, e := range rawEntries {
			// Apply activity filter if specified
			if activityFilter != "" {
				activityURI := resolveRecordURI(did, atproto.CollectionActivity, activityFilter)
				if subject := mapMap(e.Value, "subject"); subject != nil {
					if mapStr(subject, "uri") != activityURI {
						continue
					}
				}
			}
			records = append(records, map[string]any{"uri": e.URI, "record": e.Value})
		}
		fmt.Fprintln(w, prettyJSON(records))
		return nil
	}

	fmt.Fprintf(w, "\033[1m%-15s %-20s %-10s %-10s %-15s %s\033[0m\n", "ID", "METRIC", "VALUE", "UNIT", "ACTIVITY", "CREATED")
	fmt.Fprintf(w, "%-15s %-20s %-10s %-10s %-15s %s\n",
		strings.Repeat("-", 13), strings.Repeat("-", 18),
		strings.Repeat("-", 8), strings.Repeat("-", 8),
		strings.Repeat("-", 13), strings.Repeat("-", 10))

	for _, m := range entries {
		metric := m.Metric
		if len(metric) > 18 {
			metric = metric[:15] + "..."
		}
		value := m.Value
		if len(value) > 8 {
			value = value[:5] + "..."
		}
		unit := m.Unit
		if len(unit) > 8 {
			unit = unit[:5] + "..."
		}
		actRkey := m.SubjectRkey
		if len(actRkey) > 13 {
			actRkey = actRkey[:10] + "..."
		}
		if actRkey == "" {
			actRkey = "-"
		}

		fmt.Fprintf(w, "%-15s %-20s %-10s %-10s %-15s %s\n", m.Rkey, metric, value, unit, actRkey, m.Created)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "\033[90m(no measurements found)\033[0m")
	}
	return nil
}

func runMeasurementGet(ctx context.Context, cmd *cli.Command) error {
	return runSimpleGet(ctx, cmd, atproto.CollectionMeasurement, "measurement")
}
