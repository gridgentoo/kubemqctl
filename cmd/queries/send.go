package queries

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

type QueriesSendOptions struct {
	cfg       *config.Config
	transport string
	channel   string
	message   string
	metadata  string
	timeout   int
	cacheKey  string
	cacheTTL  time.Duration
}

var queriesSendExamples = `
	# Send query to a queries channel
	kubetools queries send some-channel some-query
	
	# Send query to a queries channel with metadata
	kubetools queries send some-channel some-message -m some-metadata
	
	# Send query to a queries channel with 120 seconds timeout
	kubetools queries send some-channel some-message -o 120
	
	# Send query to a queries channel with cache-key and cache duration of 1m
	kubetools queries send some-channel some-message -c cache-key -d 1m
`
var queriesSendLong = `send messages to a queries channel`
var queriesSendShort = `send messages to a queries channel`

func NewCmdQueriesSend(cfg *config.Config) *cobra.Command {
	o := &QueriesSendOptions{
		cfg: cfg,
	}
	cmd := &cobra.Command{

		Use:     "send",
		Aliases: []string{"s"},
		Short:   queriesSendShort,
		Long:    queriesSendLong,
		Example: queriesSendExamples,
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
	cmd.PersistentFlags().StringVarP(&o.cacheKey, "cache-key", "c", "", "set cache key")
	cmd.PersistentFlags().IntVarP(&o.timeout, "timeout", "o", 30, "set query timeout")
	cmd.PersistentFlags().DurationVarP(&o.cacheTTL, "cache-duration", "d", 10*time.Minute, "set cache duration timeout")

	return cmd
}

func (o *QueriesSendOptions) Complete(args []string, transport string) error {
	o.transport = transport
	if len(args) >= 2 {
		o.channel = args[0]
		o.message = args[1]
		return nil
	}
	return fmt.Errorf("missing arguments, must be 2 arguments, channel and message")
}

func (o *QueriesSendOptions) Validate() error {
	return nil
}

func (o *QueriesSendOptions) Run(ctx context.Context) error {
	client, err := kubemq.GetKubeMQClient(ctx, o.transport, o.cfg)
	if err != nil {
		return fmt.Errorf("create kubemq client, %s", err.Error())
	}

	defer func() {
		client.Close()
	}()

	msg := client.Q().
		SetChannel(o.channel).
		SetId(uuid.New().String()).
		SetBody([]byte(o.message)).
		SetMetadata(o.metadata).
		SetTimeout(time.Duration(o.timeout) * time.Second).
		SetCacheKey(o.cacheKey).
		SetCacheTTL(o.cacheTTL)

	res, err := msg.Send(ctx)
	if err != nil {
		return fmt.Errorf("sending query message, %s", err.Error())
	}
	utils.Printlnf("[channel: %s] [client id: %s] -> {id: %s, metadata: %s, body: %s, cache-hit: %t, executed: %t, executed at: %s, error: %s}", msg.Channel, msg.ClientId, msg.Id, res.Metadata, res.Body, res.CacheHit, res.Executed, res.ExecutedAt.Format("2006-01-02 15:04:05"), res.Error)
	return nil
}
