// cmd/reindex.go
package cmd

import (
	"fmt"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/search"
	"github.com/spf13/cobra"
)

var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Rebuild search index",
	Long:  `Rebuilds the full-text search index from all posts.`,
	RunE:  runReindex,
}

func init() {
	rootCmd.AddCommand(reindexCmd)
}

func runReindex(cmd *cobra.Command, args []string) error {
	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	fmt.Println("Rebuilding search index...")

	searchRepo := search.NewRepository(db)
	if err := searchRepo.RebuildIndex(); err != nil {
		return fmt.Errorf("failed to rebuild index: %w", err)
	}

	fmt.Println("Search index rebuilt successfully.")
	return nil
}
