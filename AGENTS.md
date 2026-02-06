# Agent Instructions

Hypercerts CLI (`hyper`) for managing impact claims, measurements, locations, attachments, and contributors on ATProto.

Module: `github.com/GainForest/hypercerts-cli` | Go 1.25 | CLI framework: `urfave/cli/v3`

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Build / Lint / Test

```bash
make build          # Build binary (version injected via -ldflags)
make test           # Run all unit tests: go test ./...
make test-race      # Race detector: go test -race ./...
make lint           # golangci-lint run
make fmt            # go fmt ./...
make coverage-html  # Coverage report (html)
make clean          # Remove binary + coverage artifacts
```

### Running a single test

```bash
go test -v -run TestConfirm ./internal/menu/...
go test -v -run "TestNormalizeDate/date_only" ./cmd/...
```

### Linters (all 8)

`govet`, `staticcheck`, `unused`, `ineffassign`, `misspell`, `unconvert`, `gofmt`, `goimports`

`goimports` enforced with `local-prefixes: github.com/GainForest/hypercerts-cli`

## Code Style

### Imports

Three groups separated by blank lines: (1) stdlib, (2) third-party, (3) local.

```go
import (
    "context"
    "fmt"

    "github.com/bluesky-social/indigo/atproto/syntax"
    "github.com/urfave/cli/v3"

    "github.com/GainForest/hypercerts-cli/internal/atproto"
    "github.com/GainForest/hypercerts-cli/internal/menu"
)
```

When the indigo `api/atproto` package collides with local code, alias it:

```go
comatproto "github.com/bluesky-social/indigo/api/atproto"
```

### Naming

| Element | Convention | Examples |
|---------|------------|---------|
| Files | `lowercase_underscores.go` | `auth.go`, `collections.go`, `confirm.go` |
| Types | PascalCase | `AuthSession`, `RecordEntry` |
| Exported funcs | PascalCase | `CreateRecord()`, `SingleSelect[T]()` |
| Unexported funcs | camelCase | `runActivityCreate()`, `requireAuth()` |
| Constants | PascalCase | `CollectionActivity`, `ErrCancelled` |
| Errors | `Err` prefix | `ErrNoAuthSession`, `ErrNonInteractive` |
| CLI actions | `run` + CommandName | `runAccountLogin`, `runActivityCreate` |

### Error handling

```go
// Sentinel errors at package level
var ErrNoAuthSession = errors.New("no auth session found")

// Wrap errors with context
return nil, fmt.Errorf("failed to create record: %w", err)

// Auth check pattern - used at the top of every authenticated CLI action
client, err := requireAuth(ctx, cmd)
if err != nil {
    return err  // requireAuth already wraps with user-friendly message
}
```

### Function signatures

- `context.Context` always first parameter
- CLI actions: `func runXxx(ctx context.Context, cmd *cli.Command) error`
- Use `io.Writer` parameter for testability
- Errors always last return value

### CLI output

Always write to `cmd.Root().Writer` (not `os.Stdout`) so tests can capture output:

```go
w := cmd.Root().Writer
fmt.Fprintf(w, "Created record: %s\n", uri)
```

### Terminal formatting

Interactive UI (forms, selects, confirms) uses `charmbracelet/huh` with `style.Theme()` for theming.
Activity create uses a `bubbletea` model (`activityFormModel` in `cmd/activityform.go`) with a side-by-side live preview card styled with `lipgloss`.
Status output uses ANSI codes directly: bold `\033[1m`, cyan `\033[36m`, green `\033[32m`, red `\033[31m`, yellow `\033[33m`, dim `\033[90m`, reset `\033[0m`.

### Current command tree

```
hyper
├── account login/logout/status
├── activity create/edit/delete/ls/get    # Core hypercert
├── measurement create/edit/delete/ls     # Impact metrics (alias: meas)
├── location create/edit/delete/ls        # Geographic coords (alias: loc)
├── attachment create/edit/delete/ls      # Evidence docs (alias: attach)
├── rights create/edit/delete/ls          # Licenses/rights
├── evaluation create/edit/delete/ls      # Third-party eval (alias: eval)
├── collection create/edit/delete/ls      # Project grouping (alias: coll)
├── funding create/edit/delete/ls         # Funding receipts (alias: fund)
├── workscope create/edit/delete/ls       # Work scope tags (alias: ws)
├── contributor create/edit/delete/ls     # People (alias: contrib)
└── get/ls/resolve                        # Generic record operations
```

### Activity optional fields

When creating an activity interactively, users can add these optional fields:
- `description` - longer description
- `workScope` (free-form) - scope of work as text string
- `workScopeTag` - link to a reusable scope tag via strongRef
- `startDate` / `endDate` - date range
- `contributors` - contributor references with roles and weights
- `locations` - array of strongRefs to location records
- `rights` - strongRef to rights/license record
- `image` - URI to hypercert image

## Testing patterns

- Unit tests: `*_test.go` (same package)
- Test naming: `TestXxx`, subtests with `t.Run("lowercase_with_underscores", ...)`
- Table-driven tests for utility/validation functions
- `setupTestXDG(t)` helper isolates XDG state using `t.TempDir()` + `t.Setenv()` + `xdg.Reload()`
- CLI tests use `ExecuteWithOutput([]string{...}, &buf)` to capture stdout
- `Confirm(w, r, msg)` and `ConfirmBulkDelete(w, r, count, type)` use huh in TTY, fall back to text prompts for tests
- All interactive UI uses `style.Theme()` for consistent theming

## Project structure

```
hyper/
  main.go                           # Entry point, calls cmd.Execute()
  cmd/
    root.go                         # BuildApp(), global flags, full command tree
    util.go                         # requireAuth, mapStr, buildStrongRef, buildLocationRecord, etc.
    util_test.go                    # Tests for shared helpers
    account.go                      # login / logout / status
    activity.go                     # Activity CRUD + selectActivity, cascading delete
    activityform.go                 # Bubbletea model: activity create with live preview card
    measurement.go                  # Measurement CRUD + activity linking via subject strongRef
    location.go                     # Location CRUD (LP v1.0) + selectLocation, selectLocations
    attachment.go                   # Attachment CRUD + subjects[] array, content URIs
    rights.go                       # Rights CRUD + selectRights (licenses)
    evaluation.go                   # Evaluation CRUD + score, measurements[], evaluators[]
    collection.go                   # Collection CRUD + items[] with weights
    funding.go                      # Funding receipt CRUD + payment details
    workscope.go                    # Work scope tag CRUD + hierarchy
    contributor.go                  # Contributor CRUD + selectContributor
    record.go                       # Top-level get / ls / resolve shortcuts
  internal/
    atproto/
      collections.go                # NSID constants (11 record types incl. app.certified.location)
      auth.go                       # Session persistence (~/.local/state/hc/auth-session.json)
      auth_test.go                  # Persist/load/wipe session tests
      client.go                     # CreateRecord, GetRecord, PutRecord, DeleteRecord, ListAllRecords
    menu/
      confirm.go                    # Confirm(), ConfirmBulkDelete() (huh-powered, text fallback for non-TTY)
      confirm_test.go               # Confirm/reject/auto-confirm tests
      select.go                     # SingleSelect[T], SingleSelectWithCreate[T] (generic, huh-powered)
      multiselect.go                # MultiSelect[T] (generic, checkboxes, huh-powered)
    style/
      theme.go                      # Centralized huh theme (style.Theme())
  Makefile
  .golangci.yaml
  .goreleaser.yaml
  .gitignore
```

## ATProto lexicon collections

| Collection | CLI Command | Description |
|------------|-------------|-------------|
| `org.hypercerts.claim.activity` | `activity` | Hypercert activity (core impact claim) |
| `org.hypercerts.claim.measurement` | `measurement` | Impact measurement (linked via subject strongRef) |
| `app.certified.location` | `location` | Geographic location (LP v1.0, OGC CRS84) |
| `org.hypercerts.claim.attachment` | `attachment` | Evidence attachment (linked via subjects[] array) |
| `org.hypercerts.claim.rights` | `rights` | Rights and licenses (linked via strongRef) |
| `org.hypercerts.claim.contributorInformation` | `contributor` | Contributor identity and display info |
| `org.hypercerts.claim.contributionDetails` | - | Contribution role details (embedded in activity) |
| `org.hypercerts.claim.collection` | `collection` | Collection/project grouping |
| `org.hypercerts.claim.evaluation` | `evaluation` | Third-party evaluation |
| `org.hypercerts.funding.receipt` | `funding` | Funding receipt |
| `org.hypercerts.helper.workScopeTag` | `workscope` | Work scope tag (hierarchical taxonomy) |

## Dependencies

- `bluesky-social/indigo` v0.0.0-20260202181658 (ATProto SDK: identity, repo, API clients)
- `charmbracelet/bubbletea` v1.3.6 (TUI framework: activity form with live preview)
- `charmbracelet/huh` v0.8.0 (interactive forms, selects, confirms -- all TUI)
- `charmbracelet/lipgloss` v1.1.0 (terminal styling: preview card layout)
- `urfave/cli/v3` v3.6.2 (CLI framework)
- `adrg/xdg` v0.5.3 (XDG directories)
- `joho/godotenv` v1.5.1 (auto-loads `.env`)
- `golang.org/x/term` v0.39.0 (TTY detection for huh/text fallback)

## Key technical details

- Auth session at `~/.local/state/hyper/auth-session.json` (XDG state, 0600 perms)
- All record creates use `Validate: false` for unpublished lexicons
- No runtime schema compilation -- records built with explicit typed fields
- `xdg.Reload()` required after `t.Setenv("XDG_STATE_HOME", ...)` in tests
- CLI output through `cmd.Root().Writer` (not `os.Stdout`) for testability
- Interactive prompts read from `os.Stdin`; `Confirm()` accepts `io.Reader` for testing
- Global flags (`--plc-host`, `--username`, `--password`) live on root command -- use `cmd.Root()` to access
- Version injection via `var Version string` + `-ldflags="-X github.com/GainForest/hypercerts-cli/cmd.Version=..."`
- `activity delete` cascades: finds and removes linked measurements + attachments with confirmation
- Location records use Location Protocol v1.0 (LP v1.0), OGC CRS84 coordinate reference system
- StrongRefs (uri + cid) used for linking measurements/attachments to activities
- Float values stored as strings in records (DAG-CBOR doesn't support IEEE 754 floats)

## Key patterns

### requireAuth helper

Every authenticated CLI action starts with `requireAuth`:

```go
func runActivityCreate(ctx context.Context, cmd *cli.Command) error {
    client, err := requireAuth(ctx, cmd)
    if err != nil {
        return err
    }
    // ... use client
}
```

### Interactive + non-interactive dual mode

Commands check for flags first. If no flags, fall into interactive mode. Activity create uses a bubbletea model with live preview; other commands use plain huh forms:

```go
title := cmd.String("title")
if title == "" {
    // Activity create: bubbletea model with live preview card
    result, err := runActivityForm()  // launches tea.Program with side-by-side layout
    // Other commands: plain huh form
    form := huh.NewForm(
        huh.NewGroup(huh.NewInput().Title("Title").Value(&title)),
    ).WithTheme(style.Theme())
    form.Run()
}
```

### Bubbletea form with live preview (activityform.go)

`activityFormModel` embeds a `huh.Form` and renders a live preview card alongside it using lipgloss:

```go
m := newActivityFormModel()           // creates form with .Key("fieldName") on each field
final, err := tea.NewProgram(m).Run() // launches bubbletea
fm := final.(activityFormModel)
result := fm.collectResult()          // extracts form values via m.form.GetString("fieldName")
```

Key design: uses `.Key("fieldName")` + `m.form.GetString("fieldName")` instead of `&variable` binding, because the bubbletea model needs to read values in `View()` without direct variable refs. Linked records (contributors, locations, rights) are handled post-form since they need API calls.

### Contributor select-or-create

`selectContributor` uses `SingleSelectWithCreate[T]` to show existing contributors plus a "Create new contributor..." option:

```go
selected, isCreate, err := menu.SingleSelectWithCreate(w, contributors, "contributor",
    getName, getInfo, "Create new contributor...")
if isCreate {
    return createContributorInline(ctx, client, w)
}
```

### Activity optional fields

Activity create presents all optional fields in the bubbletea form groups (Activity Details, Dates & Media, Linked Records). Users fill in what they want and skip the rest. The linked record confirms (`addContributors`, `addLocations`, `addRights`) trigger post-form API flows.

### Cascading delete

`deleteActivity` finds linked records by scanning measurement/attachment collections for matching subject URIs:

```go
measurementURIs := findLinkedURIs(ctx, client, did, atproto.CollectionMeasurement, "subject", uri)
attachmentURIs := findLinkedURIs(ctx, client, did, atproto.CollectionAttachment, "subjects", uri)
```

### StrongRef linking pattern

Measurements and attachments link to activities using strongRef (uri + cid):

```go
// Build a strongRef for record linking
ref := buildStrongRef(activityURI, activityCID)
// Returns: map[string]any{"uri": "at://...", "cid": "bafy..."}

// Use selectActivity to get URI+CID for linking
uri, cid, err := selectActivity(ctx, client, w)
record["subject"] = buildStrongRef(uri, cid)
```

### Location record pattern (LP v1.0)

Locations follow Location Protocol v1.0 with OGC CRS84:

```go
// Build a location record
record := buildLocationRecord(lat, lon, name, description)
// Sets: $type, lpVersion="1.0", srs=OGC CRS84 URI, locationType="coordinate-decimal"
// Coords stored as "lat, lon" string in location.string field

// Parse coordinates from existing record
lat, lon, ok := parseLocationCoords(record)
```

### Select-or-create pattern for locations

`selectLocation` mirrors `selectContributor` pattern:

```go
loc, err := selectLocation(ctx, client, w)  // Returns *locationOption with URI, CID, coords

// For arrays (measurement.locations[], attachment.location):
locations, err := selectLocations(ctx, client, w)  // Returns []locationOption
```

## Issue tracking (bd)

```bash
bd ready && bd show <id> && bd update <id> --status in_progress && bd close <id> && bd sync
```

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
