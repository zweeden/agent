package clicommand

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/buildkite/agent/v3/api"
	"github.com/buildkite/agent/v3/internal/stdin"
	"github.com/buildkite/agent/v3/logger"
	"github.com/buildkite/roko"
	"github.com/urfave/cli"
)

const (
	// Buildkite-imposed maximum length of annotation body (bytes).
	maxBodySize = 1024 * 1024
)

const annotateHelpDescription = `Usage:

    buildkite-agent annotate [body] [options...]

Description:

Build annotations allow you to customize the Buildkite build interface to
show information that may surface from your builds. Some examples include:

- Links to artifacts generated by your jobs
- Test result summaries
- Graphs that include analysis about your codebase
- Helpful information for team members about what happened during a build

Annotations are written in CommonMark-compliant Markdown, with "GitHub
Flavored Markdown" extensions.

The annotation body can be supplied as a command line argument, or by piping
content into the command. The maximum size of each annotation body is 1MiB.

You can update an existing annotation's body by running the annotate command
again and provide the same context as the one you want to update. Or if you
leave context blank, it will use the default context.

You can also update only the style of an existing annotation by omitting the
body entirely and providing a new style value.

Example:

    $ buildkite-agent annotate "All tests passed! :rocket:"
    $ cat annotation.md | buildkite-agent annotate --style "warning"
    $ buildkite-agent annotate --style "success" --context "junit"
    $ ./script/dynamic_annotation_generator | buildkite-agent annotate --style "success"`

type AnnotateConfig struct {
	Body    string `cli:"arg:0" label:"annotation body"`
	Style   string `cli:"style"`
	Context string `cli:"context"`
	Append  bool   `cli:"append"`
	Job     string `cli:"job" validate:"required"`

	// Global flags
	Debug       bool     `cli:"debug"`
	LogLevel    string   `cli:"log-level"`
	NoColor     bool     `cli:"no-color"`
	Experiments []string `cli:"experiment" normalize:"list"`
	Profile     string   `cli:"profile"`

	// API config
	DebugHTTP        bool   `cli:"debug-http"`
	AgentAccessToken string `cli:"agent-access-token" validate:"required"`
	Endpoint         string `cli:"endpoint" validate:"required"`
	NoHTTP2          bool   `cli:"no-http2"`
}

var AnnotateCommand = cli.Command{
	Name:        "annotate",
	Usage:       "Annotate the build page within the Buildkite UI with text from within a Buildkite job",
	Description: annotateHelpDescription,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:   "context",
			Usage:  "The context of the annotation used to differentiate this annotation from others",
			EnvVar: "BUILDKITE_ANNOTATION_CONTEXT",
		},
		cli.StringFlag{
			Name:   "style",
			Usage:  "The style of the annotation (′success′, ′info′, ′warning′ or ′error′)",
			EnvVar: "BUILDKITE_ANNOTATION_STYLE",
		},
		cli.BoolFlag{
			Name:   "append",
			Usage:  "Append to the body of an existing annotation",
			EnvVar: "BUILDKITE_ANNOTATION_APPEND",
		},
		cli.StringFlag{
			Name:   "job",
			Value:  "",
			Usage:  "Which job should the annotation come from",
			EnvVar: "BUILDKITE_JOB_ID",
		},

		// API Flags
		AgentAccessTokenFlag,
		EndpointFlag,
		NoHTTP2Flag,
		DebugHTTPFlag,

		// Global flags
		NoColorFlag,
		DebugFlag,
		LogLevelFlag,
		ExperimentsFlag,
		ProfileFlag,
	},
	Action: func(c *cli.Context) error {
		ctx := context.Background()
		ctx, cfg, l, _, done := setupLoggerAndConfig[AnnotateConfig](ctx, c)
		defer done()

		if err := annotate(ctx, cfg, l); err != nil {
			return err
		}

		return nil
	},
}

func annotate(ctx context.Context, cfg AnnotateConfig, l logger.Logger) error {
	var body string

	if cfg.Body != "" {
		body = cfg.Body
	} else if stdin.IsReadable() {
		l.Info("Reading annotation body from STDIN")

		// Actually read the file from STDIN
		stdin, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from STDIN: %w", err)
		}

		body = string(stdin[:])
	}

	if bodySize := len(cfg.Body); bodySize > maxBodySize {
		return fmt.Errorf("annotation body size (%dB) exceeds maximum (%dB)", bodySize, maxBodySize)
	}

	// Create the API client
	client := api.NewClient(l, loadAPIClientConfig(cfg, "AgentAccessToken"))

	// Create the annotation we'll send to the Buildkite API
	annotation := &api.Annotation{
		Body:    body,
		Style:   cfg.Style,
		Context: cfg.Context,
		Append:  cfg.Append,
	}

	// Retry the annotation a few times before giving up
	if err := roko.NewRetrier(
		roko.WithMaxAttempts(5),
		roko.WithStrategy(roko.Constant(1*time.Second)),
		roko.WithJitter(),
	).DoWithContext(ctx, func(r *roko.Retrier) error {
		// Attempt to create the annotation
		resp, err := client.Annotate(ctx, cfg.Job, annotation)

		// Don't bother retrying if the response was one of these statuses
		if resp != nil && (resp.StatusCode == 401 || resp.StatusCode == 404 || resp.StatusCode == 400) {
			r.Break()
			return err
		}

		// Show the unexpected error
		if err != nil {
			l.Warn("%s (%s)", err, r)
			return err
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to annotate build: %w", err)
	}

	l.Debug("Successfully annotated build")

	return nil
}
