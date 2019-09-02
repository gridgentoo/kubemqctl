package commands

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/kubemq-io/kubetools/pkg/config"
	"github.com/kubemq-io/kubetools/pkg/k8s"
	"github.com/kubemq-io/kubetools/pkg/kubemq"
	"github.com/kubemq-io/kubetools/pkg/utils"
	"github.com/spf13/cobra"
	"time"
)

type CommandsSendOptions struct {
	cfg       *config.Config
	transport string
	channel   string
	message   string
	metadata  string
	timeout   int
}

var commandsSendExamples = `
	# Send command to a commands channel
	kubetools commands send some-channel some-command
	
	# Send command to a commands channel with metadata
	kubetools commands send some-channel some-message -m some-metadata
	
	# Send command to a commands channel with 120 seconds timeout
	kubetools commands send some-channel some-message -o 120
`
var commandsSendLong = `send messages to a commands channel`
var commandsSendShort = `send messages to a commands channel`

func NewCmdCommandsSend(cfg *config.Config) *cobra.Command {
	o := &CommandsSendOptions{
		cfg: cfg,
	}
	cmd := &cobra.Command{

		Use:     "send",
		Aliases: []string{"s"},
		Short:   commandsSendShort,
		Long:    commandsSendLong,
		Example: commandsSendExamples,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			utils.CheckErr(o.Complete(args, cfg.ConnectionType))
			utils.CheckErr(o.Validate())
			utils.CheckErr(k8s.SetTransport(ctx, cfg))
			utils.CheckErr(o.Run(ctx))
		},
	}
	cmd.PersistentFlags().StringVarP(&o.metadata, "metadata", "m", "", "set metadata message")
	cmd.PersistentFlags().IntVarP(&o.timeout, "timeout", "o", 30, "set command timeout")

	return cmd
}

func (o *CommandsSendOptions) Complete(args []string, transport string) error {
	o.transport = transport
	if len(args) >= 2 {
		o.channel = args[0]
		o.message = args[1]
		return nil
	}
	return fmt.Errorf("missing arguments, must be 2 arguments, channel and message")
}

func (o *CommandsSendOptions) Validate() error {
	return nil
}

func (o *CommandsSendOptions) Run(ctx context.Context) error {
	client, err := kubemq.GetKubeMQClient(ctx, o.transport, o.cfg)
	if err != nil {
		return fmt.Errorf("create kubemq client, %s", err.Error())
	}

	defer func() {
		client.Close()
	}()

	msg := client.C().
		SetChannel(o.channel).
		SetId(uuid.New().String()).
		SetBody([]byte(o.message)).
		SetMetadata(o.metadata).
		SetTimeout(time.Duration(o.timeout) * time.Second)
	res, err := msg.Send(ctx)
	if err != nil {
		return fmt.Errorf("sending commands message, %s", err.Error())
	}
	utils.Printlnf("[channel: %s] [client id: %s] -> {id: %s, executed: %t, executed at: %s, error: %s}", msg.Channel, msg.ClientId, msg.Id, res.Executed, res.ExecutedAt.Format("2006-01-02 15:04:05"), res.Error)

	return nil
}