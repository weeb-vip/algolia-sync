package commands

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/weeb-vip/algolia-sync/internal/eventing"
)

// serveAlgoliaSyncNatsCmd is the NATS counterpart of serve-algolia-sync-kafka.
//
// A separate command rather than a flag: prod and staging run the same image,
// so the command name is what selects the transport.
var serveAlgoliaSyncNatsCmd = &cobra.Command{
	Use:   "serve-algolia-sync-nats",
	Short: "Consume Algolia sync events from NATS JetStream",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Println("Running Algolia sync eventing over NATS...")

		return eventing.EventingAlgoliaNats()
	},
}

func init() {
	rootCmd.AddCommand(serveAlgoliaSyncNatsCmd)
}
