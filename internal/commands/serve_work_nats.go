package commands

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/weeb-vip/algolia-sync/internal/eventing"
)

// serveWorkSyncNatsCmd feeds the works search index.
//
// A separate command rather than a flag on serve-algolia-sync-nats, matching
// how the transports are already selected here: prod and staging run the same
// image, and the command name is what decides which pipeline the process is.
var serveWorkSyncNatsCmd = &cobra.Command{
	Use:   "serve-work-sync-nats",
	Short: "Consume work records from NATS JetStream into the works index queue",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Println("Running work sync eventing over NATS...")

		return eventing.EventingWorkNats()
	},
}

func init() {
	rootCmd.AddCommand(serveWorkSyncNatsCmd)
}
