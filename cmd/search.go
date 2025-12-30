// cmd/search.go
package cmd

import (
	"fmt"
	"strings"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/search"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search posts by content",
	Long:  `Full-text search across all post titles and content.`,
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSearch,
}

var (
	searchLimit    int
	searchUseScore bool
)

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 20, "Maximum results to show")
	searchCmd.Flags().BoolVar(&searchUseScore, "ranked", false, "Rank by combined relevance and score")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")

	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	searchRepo := search.NewRepository(db)

	var results []search.SearchResult
	if searchUseScore {
		results, err = searchRepo.SearchWithScore(query, searchLimit)
	} else {
		results, err = searchRepo.Search(query, searchLimit)
	}

	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Printf("No results found for '%s'\n", query)
		return nil
	}

	// Display results
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	snippetStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

	fmt.Printf("\n%s '%s' (%d results)\n\n", titleStyle.Render("SEARCH:"), query, len(results))

	for _, r := range results {
		fmt.Printf("%s %s\n", idStyle.Render(fmt.Sprintf("[%d]", r.PostID)), r.Title)
		fmt.Printf("    %s", sourceStyle.Render(r.SourceName))
		if r.FinalScore > 0 {
			fmt.Printf(" • Score: %.1f", r.FinalScore)
		}
		fmt.Println()

		if r.Snippet != "" {
			// Clean up HTML tags from snippet
			snippet := strings.ReplaceAll(r.Snippet, "<b>", "")
			snippet = strings.ReplaceAll(snippet, "</b>", "")
			fmt.Printf("    %s\n", snippetStyle.Render(snippet))
		}
		fmt.Println()
	}

	return nil
}
