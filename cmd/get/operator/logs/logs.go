package logs

import (
	"context"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/kubemq-io/kubemqctl/pkg/k8s/manager/operator"

	"github.com/kubemq-io/kubemqctl/pkg/config"

	"github.com/kubemq-io/kubemqctl/pkg/k8s/client"
	"github.com/kubemq-io/kubemqctl/pkg/k8s/logs"

	"github.com/kubemq-io/kubemqctl/pkg/utils"
	"github.com/spf13/cobra"
)

type LogsOptions struct {
	cfg *config.Config
	*logs.Options
	disableColor bool
}

var logsExamples = `
	# Stream logs with selection of Kubemq operator
	kubemqctl get operator logs

	# Stream logs of all pods in default namespace
	kubemqctl get operator logs .* -n default

	# Stream logs of regex base pods with logs since 10m ago
	kubemqctl get operator logs kubemq-operator.* -s 10m

	# Stream logs of regex base pods with logs since 10m ago include the string of 'connection'
	kubemqctl get operator logs kubemq-operator.* -s 10m -i connection

	# Stream logs of regex base pods with logs exclude the string of 'error'
	kubemqctl get operator logs kubemq-operator.* -s 10m -e error

	# Stream logs of specific container
	kubemqctl get operator logs -c kubemq-operator-0
`

var (
	logsLong  = `Logs command allows to stream pods logs with powerful filtering capabilities`
	logsShort = `Stream logs of Kubemq operator pods command`
)

func NewCmdLogs(ctx context.Context, cfg *config.Config) *cobra.Command {
	o := &LogsOptions{
		cfg: cfg,
		Options: &logs.Options{
			PodQuery:       ".*",
			ContainerQuery: ".*",
			Timestamps:     false,
			Since:          0,
			Namespace:      "",
			Exclude:        nil,
			Include:        nil,
			AllNamespaces:  true,
			Selector:       "",
			Tail:           0,
			Color:          "auto",
		},
	}
	cmd := &cobra.Command{
		Use:     "logs",
		Aliases: []string{"log", "l"},
		Short:   logsShort,
		Long:    logsLong,
		Example: logsExamples,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			utils.CheckErr(o.Complete(args), cmd)
			utils.CheckErr(o.Validate())
			utils.CheckErr(o.Run(ctx))
		},
	}
	cmd.PersistentFlags().DurationVarP(&o.Options.Since, "since", "s", 0, "Set since duration time")
	cmd.PersistentFlags().StringVarP(&o.Options.Namespace, "namespace", "n", "kubemq", "Set default namespace")
	cmd.PersistentFlags().StringVarP(&o.Options.ContainerQuery, "container", "c", "kubemq-operator", "Set container regex")
	cmd.PersistentFlags().StringArrayVarP(&o.Options.Include, "include", "i", []string{}, "Set strings to include")
	cmd.PersistentFlags().StringArrayVarP(&o.Options.Exclude, "exclude", "e", []string{}, "Set strings to exclude")
	cmd.PersistentFlags().StringVarP(&o.Options.Selector, "label", "l", "", "Set label selector")
	cmd.PersistentFlags().Int64VarP(&o.Options.Tail, "tail", "t", 0, "Set how many lines to tail for each pod")
	cmd.PersistentFlags().BoolVarP(&o.disableColor, "disable-color", "", false, "Set to disable colorized output")

	return cmd
}

func (o *LogsOptions) Complete(args []string) error {
	c, err := client.NewClient(o.cfg.KubeConfigPath)
	if err != nil {
		return err
	}
	operatorManager, err := operator.NewManager(c)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		operators, err := operatorManager.GetKubemqOperatorsBundle()
		if err != nil {
			return err
		}

		if len(operators.List()) == 0 {
			goto NEXT
		}
		selection := ""
		prompt := &survey.Select{
			Renderer: survey.Renderer{},
			Message:  "Show logs for Kubemq operator:",
			Options:  operators.List(),
			Default:  operators.List()[0],
		}
		err = survey.AskOne(prompt, &selection)
		if err != nil {
			return err
		}
		pair := strings.Split(selection, "/")
		o.Options.Namespace = pair[0]
		o.Options.PodQuery = pair[1]
	}
NEXT:
	if len(args) > 0 {
		o.PodQuery = args[0]
	}
	if o.Options.Namespace != "" {
		o.Options.AllNamespaces = false
	}
	if o.disableColor {
		o.Options.Color = "never"
	}
	return nil
}

func (o *LogsOptions) Validate() error {
	return nil
}

func (o *LogsOptions) Run(ctx context.Context) error {
	c, err := client.NewClient(o.cfg.KubeConfigPath)
	if err != nil {
		return err
	}
	err = logs.Run(ctx, c, o.Options)
	if err != nil {
		return err
	}
	return nil
}
