package cmd

import (
	"context"
	"io"
	"os"
	"runtime/debug"

	_ "github.com/joho/godotenv/autoload"
	"github.com/urfave/cli/v3"
)

// version can be set at build time with -ldflags="-X github.com/GainForest/hypercerts-cli/cmd.version=X.Y.Z"
var version string

// Version returns the CLI version. Prefers ldflags, then Go module version from go install.
var Version = func() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}()

// Execute runs the CLI with os.Stdout as the output writer.
func Execute(args []string) error {
	return ExecuteWithOutput(args, os.Stdout)
}

// ExecuteWithOutput runs the CLI with a custom output writer for testability.
func ExecuteWithOutput(args []string, w io.Writer) error {
	app := BuildApp(w)
	return app.Run(context.Background(), args)
}

// BuildApp creates the root CLI command with all subcommands.
func BuildApp(w io.Writer) *cli.Command {
	return &cli.Command{
		Name:      "hc",
		Usage:     "Hypercerts CLI - manage impact claims on ATProto",
		Version:   Version,
		Writer:    w,
		ErrWriter: w,
		ExitErrHandler: func(_ context.Context, _ *cli.Command, _ error) {
			// Don't call os.Exit, let the error propagate
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "log verbosity (error, warn, info, debug)",
				Value:   "info",
				Sources: cli.EnvVars("HYPER_LOG_LEVEL", "LOG_LEVEL"),
			},
			&cli.StringFlag{
				Name:    "plc-host",
				Usage:   "PLC directory URL",
				Value:   "https://plc.directory",
				Sources: cli.EnvVars("ATP_PLC_HOST"),
			},
			&cli.StringFlag{
				Name:    "username",
				Usage:   "handle or DID (ephemeral auth)",
				Sources: cli.EnvVars("HYPER_USERNAME", "ATP_USERNAME"),
			},
			&cli.StringFlag{
				Name:    "password",
				Usage:   "app password (ephemeral auth)",
				Sources: cli.EnvVars("HYPER_PASSWORD", "ATP_PASSWORD"),
			},
		},
		Commands: []*cli.Command{
			// Top-level shortcuts
			cmdGet,
			cmdLs,
			cmdResolve,
			// Auth & Account
			cmdAccount,
			// Domain commands
			cmdActivity,
			cmdContributor,
			cmdLocation,
			cmdMeasurement,
			cmdAttachment,
			cmdRights,
			cmdEvaluation,
			cmdCollection,
			cmdFunding,
			cmdWorkScope,
		},
	}
}

// --- Top-level shortcuts ---

var cmdGet = &cli.Command{
	Name:      "get",
	Usage:     "get a record by AT-URI",
	ArgsUsage: "<at-uri>",
	Action:    runRecordGet,
}

var cmdLs = &cli.Command{
	Name:      "ls",
	Aliases:   []string{"list"},
	Usage:     "list records for an account",
	ArgsUsage: "<at-identifier>",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "collection", Usage: "filter by collection NSID"},
		&cli.BoolFlag{Name: "collections", Aliases: []string{"c"}, Usage: "list collection names only"},
	},
	Action: runRecordList,
}

var cmdResolve = &cli.Command{
	Name:      "resolve",
	Usage:     "lookup identity metadata (DID document)",
	ArgsUsage: "<at-identifier>",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "did", Aliases: []string{"d"}, Usage: "just resolve to DID"},
	},
	Action: runResolve,
}

// --- Account ---

var cmdAccount = &cli.Command{
	Name:  "account",
	Usage: "auth session and account management",
	Commands: []*cli.Command{
		{
			Name:  "login",
			Usage: "create session with PDS",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "username", Aliases: []string{"u"}, Usage: "handle or DID", Required: true, Sources: cli.EnvVars("HYPER_USERNAME", "ATP_USERNAME")},
				&cli.StringFlag{Name: "password", Aliases: []string{"p"}, Usage: "app password", Required: true, Sources: cli.EnvVars("HYPER_PASSWORD", "ATP_PASSWORD")},
				&cli.StringFlag{Name: "pds-host", Usage: "override PDS URL", Sources: cli.EnvVars("ATP_PDS_HOST")},
			},
			Action: runAccountLogin,
		},
		{
			Name:   "logout",
			Usage:  "delete current session",
			Action: runAccountLogout,
		},
		{
			Name:   "status",
			Usage:  "check auth and account status",
			Action: runAccountStatus,
		},
	},
}

// --- Activity ---

var cmdActivity = &cli.Command{
	Name:  "activity",
	Usage: "manage hypercert activities",
	Commands: []*cli.Command{
		{
			Name:  "create",
			Usage: "create a new activity (hypercert)",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "title", Usage: "activity title"},
				&cli.StringFlag{Name: "description", Usage: "short description"},
				&cli.StringFlag{Name: "start-date", Usage: "start date (RFC3339 or YYYY-MM-DD)"},
				&cli.StringFlag{Name: "end-date", Usage: "end date (RFC3339 or YYYY-MM-DD)"},
				&cli.StringFlag{Name: "work-scope", Usage: "work scope string"},
			},
			Action: runActivityCreate,
		},
		{
			Name:      "edit",
			Usage:     "edit an existing activity",
			ArgsUsage: "<id|at-uri>",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "title", Usage: "new title"},
				&cli.StringFlag{Name: "description", Usage: "new short description"},
				&cli.StringFlag{Name: "start-date", Usage: "new start date"},
				&cli.StringFlag{Name: "end-date", Usage: "new end date"},
				&cli.StringFlag{Name: "work-scope", Usage: "new work scope"},
			},
			Action: runActivityEdit,
		},
		{
			Name:  "delete",
			Usage: "delete activity and linked records",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "id", Usage: "activity ID, or select interactively"},
				&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "skip confirmation"},
			},
			Action: runActivityDelete,
		},
		{
			Name:    "ls",
			Aliases: []string{"list"},
			Usage:   "list activities",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "json", Usage: "output as JSON"},
			},
			Action: runActivityList,
		},
		{
			Name:      "get",
			Usage:     "get activity details (with backlinked records via Constellation)",
			ArgsUsage: "<id|at-uri>",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "measurements", Aliases: []string{"m"}, Usage: "show backlinked measurements"},
				&cli.BoolFlag{Name: "attachments", Aliases: []string{"a"}, Usage: "show backlinked attachments"},
				&cli.BoolFlag{Name: "evaluations", Aliases: []string{"e"}, Usage: "show backlinked evaluations"},
				&cli.BoolFlag{Name: "all", Usage: "show all backlinked records"},
				&cli.BoolFlag{Name: "json", Usage: "output backlinked records as JSON"},
			},
			Action: runActivityGet,
		},
	},
}

// --- Contributor ---

var cmdContributor = &cli.Command{
	Name:    "contributor",
	Aliases: []string{"contrib"},
	Usage:   "manage contributor records",
	Commands: []*cli.Command{
		{
			Name:  "create",
			Usage: "create a new contributor record",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "identifier", Usage: "DID or profile URI"},
				&cli.StringFlag{Name: "name", Usage: "display name (max 100 chars)"},
			},
			Action: runContributorCreate,
		},
		{
			Name:      "edit",
			Usage:     "edit a contributor record",
			ArgsUsage: "<id|at-uri>",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "identifier", Usage: "new identifier"},
				&cli.StringFlag{Name: "name", Usage: "new display name"},
			},
			Action: runContributorEdit,
		},
		{
			Name:  "delete",
			Usage: "delete contributor record(s)",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "id", Usage: "contributor ID, or select interactively"},
				&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "skip confirmation"},
			},
			Action: runContributorDelete,
		},
		{
			Name:    "ls",
			Aliases: []string{"list"},
			Usage:   "list contributor records",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "json", Usage: "output as JSON"},
			},
			Action: runContributorList,
		},
		{
			Name:      "get",
			Usage:     "get contributor details",
			ArgsUsage: "<id|at-uri>",
			Action:    runContributorGet,
		},
	},
}

// --- Measurement ---

var cmdMeasurement = &cli.Command{
	Name:    "measurement",
	Aliases: []string{"meas"},
	Usage:   "manage measurement records",
	Commands: []*cli.Command{
		{
			Name:  "create",
			Usage: "create a new measurement record",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "activity", Usage: "activity ID or AT-URI to link to"},
				&cli.StringFlag{Name: "metric", Usage: "metric being measured"},
				&cli.StringFlag{Name: "unit", Usage: "unit of measurement"},
				&cli.StringFlag{Name: "value", Usage: "measured value"},
				&cli.StringFlag{Name: "start-date", Usage: "start date (YYYY-MM-DD or RFC3339)"},
				&cli.StringFlag{Name: "end-date", Usage: "end date (YYYY-MM-DD or RFC3339)"},
				&cli.StringFlag{Name: "method-type", Usage: "methodology type"},
			},
			Action: runMeasurementCreate,
		},
		{
			Name:      "edit",
			Usage:     "edit a measurement record",
			ArgsUsage: "<id|at-uri>",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "metric", Usage: "new metric"},
				&cli.StringFlag{Name: "unit", Usage: "new unit"},
				&cli.StringFlag{Name: "value", Usage: "new value"},
				&cli.StringFlag{Name: "start-date", Usage: "new start date"},
				&cli.StringFlag{Name: "end-date", Usage: "new end date"},
			},
			Action: runMeasurementEdit,
		},
		{
			Name:  "delete",
			Usage: "delete measurement record(s)",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "id", Usage: "measurement ID, or select interactively"},
				&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "skip confirmation"},
			},
			Action: runMeasurementDelete,
		},
		{
			Name:    "ls",
			Aliases: []string{"list"},
			Usage:   "list measurement records",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "json", Usage: "output as JSON"},
				&cli.StringFlag{Name: "activity", Usage: "filter by activity ID or AT-URI"},
			},
			Action: runMeasurementList,
		},
		{
			Name:      "get",
			Usage:     "get measurement details",
			ArgsUsage: "<id|at-uri>",
			Action:    runMeasurementGet,
		},
	},
}

// --- Location ---

var cmdLocation = &cli.Command{
	Name:    "location",
	Aliases: []string{"loc"},
	Usage:   "manage location records",
	Commands: []*cli.Command{
		{
			Name:  "create",
			Usage: "create a new location record",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "lat", Usage: "latitude (-90 to 90)"},
				&cli.StringFlag{Name: "lon", Usage: "longitude (-180 to 180)"},
				&cli.StringFlag{Name: "name", Usage: "location name (optional)"},
				&cli.StringFlag{Name: "description", Usage: "location description (optional)"},
			},
			Action: runLocationCreate,
		},
		{
			Name:      "edit",
			Usage:     "edit a location record",
			ArgsUsage: "<id|at-uri>",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "lat", Usage: "new latitude"},
				&cli.StringFlag{Name: "lon", Usage: "new longitude"},
				&cli.StringFlag{Name: "name", Usage: "new name"},
				&cli.StringFlag{Name: "description", Usage: "new description"},
			},
			Action: runLocationEdit,
		},
		{
			Name:  "delete",
			Usage: "delete location record(s)",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "id", Usage: "location ID, or select interactively"},
				&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "skip confirmation"},
			},
			Action: runLocationDelete,
		},
		{
			Name:    "ls",
			Aliases: []string{"list"},
			Usage:   "list location records",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "json", Usage: "output as JSON"},
			},
			Action: runLocationList,
		},
		{
			Name:      "get",
			Usage:     "get location details",
			ArgsUsage: "<id|at-uri>",
			Action:    runLocationGet,
		},
	},
}

// --- Attachment ---

var cmdAttachment = &cli.Command{
	Name:    "attachment",
	Aliases: []string{"attach"},
	Usage:   "manage attachment records",
	Commands: []*cli.Command{
		{
			Name:  "create",
			Usage: "create a new attachment record",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "activity", Usage: "activity ID(s) to link, comma-separated"},
				&cli.StringFlag{Name: "title", Usage: "attachment title"},
				&cli.StringFlag{Name: "content-type", Usage: "content type (report, audit, evidence, testimonial, methodology)"},
				&cli.StringFlag{Name: "uri", Usage: "content URI(s), comma-separated"},
			},
			Action: runAttachmentCreate,
		},
		{
			Name:      "edit",
			Usage:     "edit an attachment record",
			ArgsUsage: "<id|at-uri>",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "title", Usage: "new title"},
				&cli.StringFlag{Name: "content-type", Usage: "new content type"},
			},
			Action: runAttachmentEdit,
		},
		{
			Name:  "delete",
			Usage: "delete attachment record(s)",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "id", Usage: "attachment ID, or select interactively"},
				&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "skip confirmation"},
			},
			Action: runAttachmentDelete,
		},
		{
			Name:    "ls",
			Aliases: []string{"list"},
			Usage:   "list attachment records",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "json", Usage: "output as JSON"},
				&cli.StringFlag{Name: "activity", Usage: "filter by activity ID or AT-URI"},
			},
			Action: runAttachmentList,
		},
		{
			Name:      "get",
			Usage:     "get attachment details",
			ArgsUsage: "<id|at-uri>",
			Action:    runAttachmentGet,
		},
	},
}

// --- Rights ---

var cmdRights = &cli.Command{
	Name:  "rights",
	Usage: "manage rights records",
	Commands: []*cli.Command{
		{
			Name:  "create",
			Usage: "create a new rights record",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "name", Usage: "rights name (max 100 chars)"},
				&cli.StringFlag{Name: "type", Usage: "rights type short ID (max 10 chars, e.g. CC-BY-4.0)"},
				&cli.StringFlag{Name: "description", Usage: "rights description"},
				&cli.StringFlag{Name: "attachment", Usage: "attachment URI (legal document)"},
			},
			Action: runRightsCreate,
		},
		{
			Name:      "edit",
			Usage:     "edit a rights record",
			ArgsUsage: "<id|at-uri>",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "name", Usage: "new rights name"},
				&cli.StringFlag{Name: "type", Usage: "new rights type"},
				&cli.StringFlag{Name: "description", Usage: "new description"},
			},
			Action: runRightsEdit,
		},
		{
			Name:  "delete",
			Usage: "delete rights record(s)",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "id", Usage: "rights ID, or select interactively"},
				&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "skip confirmation"},
			},
			Action: runRightsDelete,
		},
		{
			Name:    "ls",
			Aliases: []string{"list"},
			Usage:   "list rights records",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "json", Usage: "output as JSON"},
			},
			Action: runRightsList,
		},
		{
			Name:      "get",
			Usage:     "get rights details",
			ArgsUsage: "<id|at-uri>",
			Action:    runRightsGet,
		},
	},
}

// --- Evaluation ---

var cmdEvaluation = &cli.Command{
	Name:    "evaluation",
	Aliases: []string{"eval"},
	Usage:   "manage evaluation records",
	Commands: []*cli.Command{
		{
			Name:  "create",
			Usage: "create a new evaluation record",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "summary", Usage: "evaluation summary"},
			},
			Action: runEvaluationCreate,
		},
		{
			Name:      "edit",
			Usage:     "edit an evaluation record",
			ArgsUsage: "<id|at-uri>",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "summary", Usage: "new summary"},
			},
			Action: runEvaluationEdit,
		},
		{
			Name:  "delete",
			Usage: "delete evaluation record(s)",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "id", Usage: "evaluation ID, or select interactively"},
				&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "skip confirmation"},
			},
			Action: runEvaluationDelete,
		},
		{
			Name:    "ls",
			Aliases: []string{"list"},
			Usage:   "list evaluation records",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "json", Usage: "output as JSON"},
			},
			Action: runEvaluationList,
		},
		{
			Name:      "get",
			Usage:     "get evaluation details",
			ArgsUsage: "<id|at-uri>",
			Action:    runEvaluationGet,
		},
	},
}

// --- Collection ---

var cmdCollection = &cli.Command{
	Name:    "collection",
	Aliases: []string{"coll"},
	Usage:   "manage collection records (project groupings)",
	Commands: []*cli.Command{
		{
			Name:  "create",
			Usage: "create a new collection",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "title", Usage: "collection title"},
				&cli.StringFlag{Name: "type", Usage: "collection type (e.g. project, favorites)"},
				&cli.StringFlag{Name: "description", Usage: "short description"},
			},
			Action: runCollectionCreate,
		},
		{
			Name:      "edit",
			Usage:     "edit a collection",
			ArgsUsage: "<id|at-uri>",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "title", Usage: "new title"},
				&cli.StringFlag{Name: "type", Usage: "new type"},
			},
			Action: runCollectionEdit,
		},
		{
			Name:  "delete",
			Usage: "delete collection(s)",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "id", Usage: "collection ID, or select interactively"},
				&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "skip confirmation"},
			},
			Action: runCollectionDelete,
		},
		{
			Name:    "ls",
			Aliases: []string{"list"},
			Usage:   "list collections",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "json", Usage: "output as JSON"},
			},
			Action: runCollectionList,
		},
		{
			Name:      "get",
			Usage:     "get collection details",
			ArgsUsage: "<id|at-uri>",
			Action:    runCollectionGet,
		},
	},
}

// --- Funding ---

var cmdFunding = &cli.Command{
	Name:    "funding",
	Aliases: []string{"fund"},
	Usage:   "manage funding receipts",
	Commands: []*cli.Command{
		{
			Name:  "create",
			Usage: "create a new funding receipt",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "from", Usage: "sender DID"},
				&cli.StringFlag{Name: "to", Usage: "recipient (DID or name)"},
				&cli.StringFlag{Name: "amount", Usage: "amount"},
				&cli.StringFlag{Name: "currency", Usage: "currency (USD, EUR, ETH)"},
				&cli.StringFlag{Name: "rail", Usage: "payment rail (bank_transfer, credit_card, onchain, cash)"},
				&cli.StringFlag{Name: "network", Usage: "payment network (arbitrum, ethereum, sepa, visa)"},
				&cli.StringFlag{Name: "tx-id", Usage: "transaction ID"},
				&cli.StringFlag{Name: "for", Usage: "activity ID to link to"},
				&cli.StringFlag{Name: "notes", Usage: "additional notes"},
			},
			Action: runFundingCreate,
		},
		{
			Name:      "edit",
			Usage:     "edit a funding receipt",
			ArgsUsage: "<id|at-uri>",
			Action:    runFundingEdit,
		},
		{
			Name:  "delete",
			Usage: "delete funding receipt(s)",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "id", Usage: "funding receipt ID, or select interactively"},
				&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "skip confirmation"},
			},
			Action: runFundingDelete,
		},
		{
			Name:    "ls",
			Aliases: []string{"list"},
			Usage:   "list funding receipts",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "json", Usage: "output as JSON"},
				&cli.StringFlag{Name: "activity", Usage: "filter by activity ID or AT-URI"},
			},
			Action: runFundingList,
		},
		{
			Name:      "get",
			Usage:     "get funding receipt details",
			ArgsUsage: "<id|at-uri>",
			Action:    runFundingGet,
		},
	},
}

// --- Work Scope ---

var cmdWorkScope = &cli.Command{
	Name:    "workscope",
	Aliases: []string{"ws"},
	Usage:   "manage work scope tags",
	Commands: []*cli.Command{
		{
			Name:  "create",
			Usage: "create a new work scope tag",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "key", Usage: "lowercase-hyphenated key (e.g. climate-action)"},
				&cli.StringFlag{Name: "label", Usage: "human-readable label"},
				&cli.StringFlag{Name: "kind", Usage: "kind: topic, language, domain, method, tag"},
				&cli.StringFlag{Name: "description", Usage: "description"},
				&cli.StringFlag{Name: "parent", Usage: "parent tag ID for hierarchy"},
			},
			Action: runWorkScopeCreate,
		},
		{
			Name:      "edit",
			Usage:     "edit a work scope tag",
			ArgsUsage: "<id|at-uri>",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "key", Usage: "new key"},
				&cli.StringFlag{Name: "label", Usage: "new label"},
				&cli.StringFlag{Name: "kind", Usage: "new kind"},
				&cli.StringFlag{Name: "description", Usage: "new description"},
			},
			Action: runWorkScopeEdit,
		},
		{
			Name:  "delete",
			Usage: "delete work scope tag(s)",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "id", Usage: "work scope tag ID, or select interactively"},
				&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "skip confirmation"},
			},
			Action: runWorkScopeDelete,
		},
		{
			Name:    "ls",
			Aliases: []string{"list"},
			Usage:   "list work scope tags",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "json", Usage: "output as JSON"},
				&cli.StringFlag{Name: "kind", Usage: "filter by kind"},
			},
			Action: runWorkScopeList,
		},
		{
			Name:      "get",
			Usage:     "get work scope tag details",
			ArgsUsage: "<id|at-uri>",
			Action:    runWorkScopeGet,
		},
	},
}
