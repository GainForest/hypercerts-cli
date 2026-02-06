# Hypercerts CLI

A command-line tool for managing [Hypercerts](https://hypercerts.org) on the [AT Protocol](https://atproto.com). Create, edit, and manage impact claims, measurements, locations, attachments, and contributors with interactive menus or scriptable flags.

Built on [bluesky-social/indigo](https://github.com/bluesky-social/indigo) patterns with interactive terminal UI, designed for both human operators and CI/CD pipelines.

## Features

- **Full CRUD** for activities, measurements, locations, attachments, evaluations, and contributors
- **Interactive menus** with arrow key navigation and multi-select
- **Scriptable** - all commands accept flags for automation
- **Linked records** - measurements and attachments link to activities via strongRefs
- **Location Protocol v1.0** - geographic coordinates following the certified location standard
- **Cascading deletes** - removing an activity cleans up linked measurements and attachments
- **Session persistence** - auth tokens stored securely with automatic refresh

## Install

### Quick Install (with confirmation message)

```bash
curl -sSL https://raw.githubusercontent.com/GainForest/hypercerts-cli/main/install.sh | bash
```

### Manual Install

```bash
go install github.com/GainForest/hypercerts-cli/cmd/hc@v0.1.0
```

After installation, the `hc` command will be available in your terminal.

### Build from source

```bash
git clone https://github.com/GainForest/hypercerts-cli
cd hypercerts-cli
make build
./hc --help
```

### Requirements

- Go 1.25+

## Quick Start

```bash
# 1. Login to your PDS
hc account login -u yourhandle.example.com -p your-app-password

# 2. Create an activity (the core hypercert)
hc activity create \
  --title "Rainforest Carbon Study" \
  --description "12-month carbon sequestration measurement"

# 3. Add a measurement linked to the activity
hc measurement create \
  --metric "carbon sequestered" \
  --unit "tonnes CO2" \
  --value "1500"

# 4. Add a location for the measurement
hc location create \
  --lat -3.4653 \
  --lon -62.2159 \
  --name "Amazon Basin Site A"

# 5. Attach evidence
hc attachment create \
  --title "Field Report Q1" \
  --uri "https://example.com/reports/q1-2025.pdf"

# 6. List everything
hc activity ls
hc measurement ls
hc location ls
hc attachment ls
```

## Command Reference

### Account Management

```bash
hc account login -u handle -p password     # Create auth session
hc account login --pds-host https://pds.example  # Direct PDS override
hc account logout                           # Delete session
hc account status                           # Show DID, handle, PDS
```

### Activities

Activities are the core hypercert record (`org.hypercerts.claim.activity`).

```bash
# Create (interactive - prompts for fields)
hc activity create

# Create (non-interactive)
hc activity create \
  --title "Ocean Cleanup Initiative" \
  --description "Removing plastic from coastal waters" \
  --start-date 2025-01-01 \
  --end-date 2025-12-31 \
  --work-scope "ocean-cleanup"

# List activities (shows linked measurement count)
hc activity ls                # Table view
hc activity ls --json         # JSON output

# Get activity details (includes linked measurements & attachments)
hc activity get <rkey>

# Edit (interactive or with flags)
hc activity edit <rkey>
hc activity edit <rkey> --title "Updated Title"

# Delete (cascades to linked measurements & attachments)
hc activity delete              # Interactive multi-select
hc activity delete <rkey>       # Direct
hc activity delete <rkey> -f    # Skip confirmation
```

### Measurements

Measurements record impact metrics linked to activities (`org.hypercerts.claim.measurement`).

```bash
# Create (interactive - select activity, enter metrics)
hc measurement create

# Create (non-interactive)
hc measurement create \
  --activity <activity-rkey> \
  --metric "trees planted" \
  --unit "count" \
  --value "5000" \
  --start-date 2025-01-01 \
  --end-date 2025-06-30

# List all measurements
hc measurement ls
hc measurement ls --json

# Filter by activity
hc measurement ls --activity <activity-rkey>

# Edit
hc measurement edit <rkey>
hc measurement edit <rkey> --value "5500"

# Delete
hc measurement delete              # Interactive multi-select
hc measurement delete <rkey> -f    # Direct with force
```

**Alias**: `hc meas` works as shorthand.

### Locations

Geographic locations following Location Protocol v1.0 (`app.certified.location`).

```bash
# Create (interactive - prompts for coordinates)
hc location create

# Create (non-interactive)
hc location create \
  --lat 47.6062 \
  --lon -122.3321 \
  --name "Seattle HQ" \
  --description "Main research facility"

# List locations
hc location ls
hc location ls --json

# Edit
hc location edit <rkey>
hc location edit <rkey> --name "Seattle Office"

# Delete
hc location delete              # Interactive multi-select
hc location delete <rkey> -f    # Direct with force
```

**Alias**: `hc loc` works as shorthand.

**Technical details**: Locations use LP v1.0 with OGC CRS84 coordinate reference system. Coordinates are stored as `"lat, lon"` string in the `location.string` field with `locationType: "coordinate-decimal"`.

### Attachments

Evidence attachments linked to activities (`org.hypercerts.claim.attachment`).

```bash
# Create (interactive - select activities, enter URIs)
hc attachment create

# Create (non-interactive)
hc attachment create \
  --activity <activity-rkey> \
  --title "Audit Report 2025" \
  --content-type report \
  --uri "https://example.com/audit-2025.pdf"

# Multiple activities and URIs (comma-separated)
hc attachment create \
  --activity "rkey1,rkey2" \
  --title "Shared Evidence" \
  --uri "https://example.com/doc1.pdf,https://example.com/doc2.pdf"

# List all attachments
hc attachment ls
hc attachment ls --json

# Filter by activity
hc attachment ls --activity <activity-rkey>

# Edit
hc attachment edit <rkey>
hc attachment edit <rkey> --title "Updated Report"

# Delete
hc attachment delete              # Interactive multi-select
hc attachment delete <rkey> -f    # Direct with force
```

**Alias**: `hc attach` works as shorthand.

**Content types**: `report`, `audit`, `evidence`, `testimonial`, `methodology`

### Rights

Rights and license definitions for hypercerts (`org.hypercerts.claim.rights`).

```bash
# Create (interactive)
hc rights create

# Create (non-interactive)
hc rights create \
  --name "Creative Commons BY 4.0" \
  --type "CC-BY-4.0" \
  --description "Attribution required, commercial use allowed"

# List rights
hc rights ls
hc rights ls --json

# Edit
hc rights edit <rkey>
hc rights edit <rkey> --name "Updated Name"

# Delete
hc rights delete              # Interactive multi-select
hc rights delete <rkey> -f    # Direct with force
```

Rights can be linked to activities during creation via the optional fields menu.

### Evaluations

Third-party evaluations for impact claims (`org.hypercerts.claim.evaluation`).

```bash
# Create (interactive - select activity, add evaluators, score)
hc evaluation create

# Create (non-interactive)
hc evaluation create \
  --summary "Independent verification of carbon sequestration claims"

# List evaluations
hc evaluation ls
hc evaluation ls --json

# Edit
hc evaluation edit <rkey>
hc evaluation edit <rkey> --summary "Updated evaluation summary"

# Delete
hc evaluation delete              # Interactive multi-select
hc evaluation delete <rkey> -f    # Direct with force
```

**Alias**: `hc eval` works as shorthand.

**Fields**:
- `evaluators[]` - Array of evaluator DIDs (required)
- `summary` - Brief evaluation summary (required)
- `subject` - StrongRef to the record being evaluated (optional)
- `content[]` - Array of content URIs (reports, methodology docs)
- `measurements[]` - Array of strongRefs to measurement records
- `score` - Score object with `min`, `max`, and `value`
- `location` - StrongRef to location record

### Collections

Project groupings of activities (`org.hypercerts.claim.collection`).

```bash
# Create (interactive - select activities to include)
hc collection create

# Create (non-interactive)
hc collection create \
  --title "Amazon Conservation 2025" \
  --type "project" \
  --description "All activities related to Amazon conservation"

# List collections
hc collection ls
hc collection ls --json

# Edit
hc collection edit <rkey>
hc collection edit <rkey> --title "Updated Project Name"

# Delete
hc collection delete              # Interactive multi-select
hc collection delete <rkey> -f    # Direct with force
```

**Alias**: `hc coll` works as shorthand.

**Fields**:
- `title` - Collection title (required, max 80 graphemes)
- `type` - Collection type (optional, e.g. "project", "favorites")
- `items[]` - Array of items with `itemIdentifier` (strongRef) and optional `itemWeight`
- `shortDescription` - Brief summary (optional, max 300 graphemes)
- `location` - StrongRef to location record (optional)

### Funding Receipts

Funding receipts for tracking payments (`org.hypercerts.funding.receipt`).

```bash
# Create (interactive - prompts for payment details)
hc funding create

# Create (non-interactive)
hc funding create \
  --from did:plc:sender123 \
  --to did:plc:recipient456 \
  --amount "1000.00" \
  --currency "USD" \
  --for <activity-rkey>

# List funding receipts
hc funding ls
hc funding ls --json
hc funding ls --activity <activity-rkey>  # Filter by linked activity

# Edit
hc funding edit <rkey>

# Delete
hc funding delete              # Interactive multi-select
hc funding delete <rkey> -f    # Direct with force
```

**Alias**: `hc fund` works as shorthand.

**Fields**:
- `from` - Sender DID (required)
- `to` - Recipient DID or name (required)
- `amount` - Payment amount as string (required)
- `currency` - Currency code: USD, EUR, ETH, etc. (required)
- `paymentRail` - Payment method: bank_transfer, credit_card, onchain, cash (optional)
- `paymentNetwork` - Network: arbitrum, ethereum, sepa, visa, paypal (optional)
- `transactionId` - Payment transaction reference (optional)
- `for` - AT-URI of linked activity (optional)
- `notes` - Additional context, max 500 chars (optional)
- `occurredAt` - When payment occurred (optional)

### Work Scope Tags

Reusable work scope tags with hierarchy (`org.hypercerts.helper.workScopeTag`).

```bash
# Create (interactive)
hc workscope create

# Create (non-interactive)
hc workscope create \
  --key "climate-action" \
  --label "Climate Action" \
  --kind "topic" \
  --description "Projects related to climate change mitigation"

# List work scope tags
hc workscope ls
hc workscope ls --json
hc workscope ls --kind topic  # Filter by kind

# Edit
hc workscope edit <rkey>
hc workscope edit <rkey> --label "Updated Label"

# Delete
hc workscope delete              # Interactive multi-select
hc workscope delete <rkey> -f    # Direct with force
```

**Alias**: `hc ws` works as shorthand.

**Fields**:
- `key` - Lowercase-hyphenated machine key (required, max 120 chars)
- `label` - Human-readable label (required, max 200 chars)
- `kind` - Category: topic, language, domain, method, tag (optional)
- `description` - Longer description (optional, max 1000 graphemes)
- `parent` - StrongRef to parent tag for hierarchy (optional)
- `aliases[]` - Alternative names (optional, max 50 items)

### Contributors

Contributor identity records (`org.hypercerts.claim.contributorInformation`).

```bash
# Create
hc contributor create
hc contributor create --identifier did:plc:abc123 --name "Alice Chen"

# List
hc contributor ls
hc contributor ls --json

# Edit
hc contributor edit <rkey>
hc contributor edit <rkey> --name "Alice M. Chen"

# Delete
hc contributor delete
hc contributor delete <rkey> -f
```

**Alias**: `hc contrib` works as shorthand.

### Generic Record Operations

```bash
# Get any ATProto record by AT-URI
hc get at://did:plc:xxx/org.hypercerts.claim.activity/rkey

# List all records for an account
hc ls handle.example.com
hc ls handle.example.com --collection org.hypercerts.claim.activity
hc ls handle.example.com -c   # List collection names only

# Resolve identity (DID document)
hc resolve handle.example.com
hc resolve -d handle.example.com   # Just the DID
```

## Data Model

```
                    ┌─────────────────┐
                    │    Activity     │
                    │  (hypercert)    │
                    └────────┬────────┘
                             │
     ┌───────────┬───────────┼───────────┬───────────┬───────────┐
     │           │           │           │           │           │
     ▼           ▼           ▼           ▼           ▼           ▼
┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐
│Measuremt│ │Attachmnt│ │Contribut│ │ Rights  │ │Location │ │Evaluatn │
│(metrics)│ │(evidence│ │ (people)│ │(license)│ │ (geo)   │ │(3rd pty)│
└────┬────┘ └────┬────┘ └─────────┘ └─────────┘ └─────────┘ └────┬────┘
     │           │                                               │
     ▼           ▼                                               ▼
┌─────────┐ ┌─────────┐                                    ┌─────────┐
│Location │ │Location │                                    │Measuremt│
│(optional│ │(optional│                                    │(linked) │
└─────────┘ └─────────┘                                    └─────────┘
```

**Relationships**:
- Measurements link to activities via `subject` (strongRef)
- Attachments link to activities via `subjects[]` (array of strongRefs)
- Contributors are embedded in activities via `contributors[]`
- Rights link to activities via `rights` (strongRef)
- Locations link to activities via `locations[]` (array of strongRefs)
- Locations can also be linked to measurements and attachments
- Evaluations link to activities via `subject` and can include `measurements[]`

## Interactive Menus

When commands are run without an ID argument, `hc` shows interactive menus:

**Single selection** (for edit):
```
Select an activity:

  > Rainforest Carbon Study        (2025-06-15)
    Ocean Cleanup Initiative       (2025-03-01)

↑/↓ navigate · enter select · q cancel
```

**Multi-selection** (for delete):
```
Select activities: 2 selected

  > [x] Rainforest Carbon Study    (2025-06-15)
    [x] Ocean Cleanup Initiative   (2025-03-01)
    [ ] Urban Garden Project       (2025-04-15)

↑/↓ navigate · space toggle · a all · enter confirm · q cancel
```

**Select or create** (when linking contributors):
```
Select a contributor:

  > Alice Chen                     (did:plc:abc123)
    Bob Martinez                   (did:plc:xyz789)

  + Create new contributor...

↑/↓ navigate · enter select · q cancel
```

## Lexicon Collections

| Collection | CLI Command | Description |
|------------|-------------|-------------|
| `org.hypercerts.claim.activity` | `activity` | Core hypercert / impact claim |
| `org.hypercerts.claim.measurement` | `measurement` | Impact metrics linked to activity |
| `app.certified.location` | `location` | Geographic location (LP v1.0) |
| `org.hypercerts.claim.attachment` | `attachment` | Evidence documents/URIs |
| `org.hypercerts.claim.rights` | `rights` | Rights and licenses |
| `org.hypercerts.claim.contributorInformation` | `contributor` | Contributor identity |
| `org.hypercerts.claim.contributionDetails` | - | Contribution role (embedded in activity) |
| `org.hypercerts.claim.collection` | `collection` | Project grouping |
| `org.hypercerts.claim.evaluation` | `evaluation` | Third-party evaluation |
| `org.hypercerts.funding.receipt` | `funding` | Funding receipt |
| `org.hypercerts.helper.workScopeTag` | `workscope` | Work scope taxonomy tag |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `HYPER_USERNAME` | Handle or DID for auth |
| `HYPER_PASSWORD` | App password for auth |
| `ATP_PDS_HOST` | Override PDS URL |
| `ATP_PLC_HOST` | Override PLC directory URL (default: `https://plc.directory`) |
| `HYPER_LOG_LEVEL` | Log level: error, warn, info, debug |

Environment variables can also be set in a `.env` file in the working directory.

## Development

### Build & Test

```bash
make build          # Build binary (version injected via ldflags)
make test           # Run all unit tests
make test-race      # Run with race detector
make lint           # golangci-lint run
make fmt            # Format code
make coverage-html  # Generate coverage report
make clean          # Remove binary + coverage artifacts
```

### Running Specific Tests

```bash
# Single test
go test -v -run TestConfirm ./internal/menu/...

# Test with pattern
go test -v -run "TestNormalizeDate/date_only" ./cmd/...

# All tests in a package
go test -v ./cmd/...
```

### Project Structure

```
hypercerts-cli/
├── cmd/hc/main.go                   # Entry point
├── Makefile                         # Build targets
├── go.mod, go.sum                   # Dependencies
│
├── cmd/
│   ├── root.go                      # BuildApp(), global flags, command tree
│   ├── util.go                      # Shared helpers (requireAuth, mapStr, buildStrongRef, etc.)
│   ├── util_test.go                 # Tests for helpers
│   ├── account.go                   # login / logout / status
│   ├── activity.go                  # Activity CRUD + selectActivity, cascading delete
│   ├── measurement.go               # Measurement CRUD + selectActivity linking
│   ├── location.go                  # Location CRUD + selectLocation, selectLocations
│   ├── attachment.go                # Attachment CRUD + content URIs, subjects[]
│   ├── rights.go                    # Rights CRUD + selectRights (licenses)
│   ├── evaluation.go                # Evaluation CRUD + score, measurements[]
│   ├── collection.go                # Collection CRUD + items[] with weights
│   ├── funding.go                   # Funding receipt CRUD + payment details
│   ├── workscope.go                 # Work scope tag CRUD + hierarchy
│   ├── contributor.go               # Contributor CRUD + selectContributor
│   └── record.go                    # Top-level get / ls / resolve
│
└── internal/
    ├── atproto/
    │   ├── collections.go           # NSID constants (11 record types)
    │   ├── auth.go                  # Session persistence (~/.local/state/hc/)
    │   ├── auth_test.go             # Auth tests
    │   └── client.go                # Record CRUD (Create/Get/Put/Delete/ListAll)
    │
    ├── menu/
    │   ├── confirm.go               # Confirm(), ConfirmBulkDelete()
    │   ├── confirm_test.go
    │   ├── select.go                # SingleSelect[T], SingleSelectWithCreate[T]
    │   ├── multiselect.go           # MultiSelect[T]
    │   ├── render.go                # ANSI rendering
    │   └── render_test.go
    │
    └── prompt/
        ├── prompt.go                # ReadLine, ReadLineWithDefault, ReadOptionalField
        └── prompt_test.go
```

### Key Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| `bluesky-social/indigo` | v0.0.0-20260202 | ATProto SDK (identity, repo, API) |
| `urfave/cli/v3` | v3.6.2 | CLI framework |
| `adrg/xdg` | v0.5.3 | XDG directories |
| `joho/godotenv` | v1.5.1 | `.env` file loading |
| `golang.org/x/term` | v0.39.0 | Raw terminal for menus |

### Adding a New Record Type

1. Add NSID constant to `internal/atproto/collections.go`
2. Create `cmd/<type>.go` with:
   - `type <type>Option struct { ... }` for menu display
   - `fetch<Type>s()` to list records
   - `select<Type>()` for interactive selection (if needed)
   - `run<Type>Create/Edit/Delete/List()` CLI actions
3. Add `cmd<Type>` variable to `cmd/root.go`
4. Wire into `BuildApp()` Commands list
5. Add tests to `cmd/<type>_test.go`
6. Update this README

### Code Style

**Imports**: Three groups separated by blank lines: stdlib, third-party, local.

```go
import (
    "context"
    "fmt"

    "github.com/urfave/cli/v3"

    "github.com/GainForest/hypercerts-cli/internal/atproto"
)
```

**Naming**:
- Files: `lowercase_underscores.go`
- Exported: `PascalCase`
- Unexported: `camelCase`
- CLI actions: `runXxxCreate`, `runXxxEdit`, etc.

**Output**: Always use `cmd.Root().Writer` (not `os.Stdout`) for testability.

## Troubleshooting

### "not logged in" error

```bash
hc account status   # Check current session
hc account login -u handle -p password   # Re-authenticate
```

### "activity not found" when creating measurement

Measurements require an activity to link to. Create an activity first:

```bash
hc activity create --title "My Activity" --description "Description"
hc measurement create --activity <rkey-from-above>
```

### Invalid latitude/longitude

Coordinates must be valid:
- Latitude: -90 to 90
- Longitude: -180 to 180

```bash
# Valid
hc location create --lat 47.6062 --lon -122.3321

# Invalid (will error)
hc location create --lat 100 --lon -200
```

### Session expired

Sessions are stored at `~/.local/state/hc/auth-session.json`. If expired:

```bash
hc account logout
hc account login -u handle -p password
```

## License

See [LICENSE](LICENSE) for details.
