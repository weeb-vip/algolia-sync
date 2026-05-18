package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// migrateAlgoliaDataCmd transforms JSON exported from Algolia to fix genres/studios/etc from JSON strings to arrays
var migrateAlgoliaDataCmd = &cobra.Command{
	Use:   "migrate-algolia-data [input.json] [output.json]",
	Short: "Convert Algolia export data from JSON strings to arrays",
	Long: `Reads an Algolia JSON export and converts genres, studios, licensors,
and title_synonyms fields from JSON strings to proper arrays for faceting.

Usage:
  1. Export your Algolia index to JSON via the Algolia dashboard
  2. Run: algolia-sync migrate-algolia-data input.json output.json
  3. Import output.json back to Algolia via the dashboard`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputFile := args[0]
		outputFile := args[1]

		// Read input file
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}

		// Parse as array of generic objects
		var records []map[string]interface{}
		if err := json.Unmarshal(data, &records); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}

		fmt.Printf("Processing %d records...\n", len(records))

		fieldsToConvert := []string{"genres", "studios", "licensors", "title_synonyms"}
		convertedCount := 0

		for i, record := range records {
			modified := false
			for _, field := range fieldsToConvert {
				if val, exists := record[field]; exists {
					if strVal, isString := val.(string); isString && strVal != "" {
						// Parse JSON string to array
						var arr []string
						if err := json.Unmarshal([]byte(strVal), &arr); err == nil {
							record[field] = arr
							modified = true
						}
					}
				}
			}
			if modified {
				convertedCount++
			}
			records[i] = record
		}

		fmt.Printf("Converted %d records with JSON string fields to arrays\n", convertedCount)

		// Write output file
		output, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal output: %w", err)
		}

		if err := os.WriteFile(outputFile, output, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		fmt.Printf("Successfully wrote migrated data to %s\n", outputFile)
		fmt.Println("\nNext steps:")
		fmt.Println("1. Go to Algolia dashboard -> your index -> Import/Export")
		fmt.Println("2. Clear the existing index (or create a new one)")
		fmt.Println("3. Import the output JSON file")
		fmt.Println("4. Configure 'genres', 'studios', 'licensors', 'title_synonyms' as filterableAttributes")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrateAlgoliaDataCmd)
}
