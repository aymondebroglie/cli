package list

import (
	"fmt"
	"net/http"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmd/workflow/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/cli/cli/v2/utils"
	"github.com/spf13/cobra"
)

const defaultLimit = 50

type ListOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (ghrepo.Interface, error)

	PlainOutput bool

	All   bool
	Limit int
}

func NewCmdList(f *cmdutil.Factory, runF func(*ListOptions) error) *cobra.Command {
	opts := &ListOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
	}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List workflows",
		Long:    "List workflow files, hiding disabled workflows by default.",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// support `-R, --repo` override
			opts.BaseRepo = f.BaseRepo

			terminal := opts.IO.IsStdoutTTY() && opts.IO.IsStdinTTY()
			opts.PlainOutput = !terminal

			if opts.Limit < 1 {
				return cmdutil.FlagErrorf("invalid limit: %v", opts.Limit)
			}

			if runF != nil {
				return runF(opts)
			}

			return listRun(opts)
		},
	}

	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", defaultLimit, "Maximum number of workflows to fetch")
	cmd.Flags().BoolVarP(&opts.All, "all", "a", false, "Show all workflows, including disabled workflows")

	return cmd
}

func listRun(opts *ListOptions) error {
	repo, err := opts.BaseRepo()
	if err != nil {
		return fmt.Errorf("could not determine base repo: %w", err)
	}

	httpClient, err := opts.HttpClient()
	if err != nil {
		return fmt.Errorf("could not create http client: %w", err)
	}
	client := api.NewClientFromHTTP(httpClient)

	opts.IO.StartProgressIndicator()
	workflows, err := shared.GetWorkflows(client, repo, opts.Limit)
	opts.IO.StopProgressIndicator()
	if err != nil {
		return fmt.Errorf("could not get workflows: %w", err)
	}

	if len(workflows) == 0 {
		return cmdutil.NewNoResultsError("no workflows found")
	}

	if err := opts.IO.StartPager(); err == nil {
		defer opts.IO.StopPager()
	} else {
		fmt.Fprintf(opts.IO.ErrOut, "failed to start pager: %v\n", err)
	}

	//nolint:staticcheck // SA1019: utils.NewTablePrinter is deprecated: use internal/tableprinter
	tp := utils.NewTablePrinter(opts.IO)
	cs := opts.IO.ColorScheme()

	for _, workflow := range workflows {
		if workflow.Disabled() && !opts.All {
			continue
		}
		tp.AddField(workflow.Name, nil, cs.Bold)
		tp.AddField(string(workflow.State), nil, nil)
		tp.AddField(fmt.Sprintf("%d", workflow.ID), nil, cs.Cyan)
		tp.EndRow()
	}

	return tp.Render()
}
