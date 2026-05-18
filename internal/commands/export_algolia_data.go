package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/algolia/algoliasearch-client-go/v3/algolia/search"
	"github.com/spf13/cobra"
	"github.com/weeb-vip/algolia-sync/config"
)

// exportAlgoliaDataCmd exports all records from Algolia index to JSON
var exportAlgoliaDataCmd = &cobra.Command{
	Use:   "export-algolia-data [output.json]",
	Short: "Export all records from Algolia index to JSON",
	Long: `Fetches all records from the configured Algolia index using the browse API
and writes them to a JSON file. Use this before running migrate-algolia-data.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outputFile := args[0]
		cfg := config.LoadConfigOrPanic()
		ctx := context.Background()

		client := search.NewClient(cfg.AlgoliaConfig.AppID, cfg.AlgoliaConfig.APIKey)
		index := client.InitIndex(cfg.AlgoliaConfig.Index)

		fmt.Printf("Exporting from index: %s\n", cfg.AlgoliaConfig.Index)

		var records []map[string]interface{}

		res, err := index.BrowseObjects(ctx)
		if err != nil {
			return fmt.Errorf("failed to browse index: %w", err)
		}

		for {
			var record map[string]interface{}
			_, err := res.Next(&record)
			if err != nil {
				break
			}
			records = append(records, record)

			if len(records)%1000 == 0 {
				fmt.Printf("Fetched %d records...\n", len(records))
			}
		}

		fmt.Printf("Total records fetched: %d\n", len(records))

		output, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}

		if err := os.WriteFile(outputFile, output, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		fmt.Printf("Successfully exported to %s\n", outputFile)
		fmt.Println("\nNext step: run migrate-algolia-data to convert JSON strings to arrays")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(exportAlgoliaDataCmd)
}
