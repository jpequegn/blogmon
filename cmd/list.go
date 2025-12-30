// cmd/list.go
package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List posts",
	Long:  `List posts from monitored blogs, sorted by date.`,
	RunE:  runList,
}

var (
	listTop   int
	listSince string
)

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().IntVarP(&listTop, "top", "n", 20, "Number of posts to show")
	listCmd.Flags().StringVar(&listSince, "since", "", "Show posts since date (YYYY-MM-DD)")
}

func runList(cmd *cobra.Command, args []string) error {
	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	repo := post.NewRepository(db)
	posts, err := repo.List(listTop, 0)
	if err != nil {
		return err
	}

	if len(posts) == 0 {
		fmt.Println("No posts found. Run 'blogmon fetch' to download posts.")
		return nil
	}

	// Styles
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	scoreStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	dateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	sourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14"))

	// Header
	fmt.Println(headerStyle.Render(fmt.Sprintf(" %-4s  %-5s  %-10s  %-20s  %s", "#", "SCORE", "DATE", "SOURCE", "TITLE")))
	fmt.Println(strings.Repeat("─", 100))

	for i, p := range posts {
		score := "-"
		if p.FinalScore != nil && *p.FinalScore > 0 {
			score = fmt.Sprintf("%.0f", *p.FinalScore)
		}

		date := "-"
		if p.PublishedAt != nil {
			date = p.PublishedAt.Format("2006-01-02")
		}

		sourceName := p.SourceName
		if len(sourceName) > 20 {
			sourceName = sourceName[:17] + "..."
		}

		title := p.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}

		fmt.Printf(" %s  %s  %s  %s  %s\n",
			idStyle.Render(fmt.Sprintf("%-4d", p.ID)),
			scoreStyle.Render(fmt.Sprintf("%-5s", score)),
			dateStyle.Render(fmt.Sprintf("%-10s", date)),
			sourceStyle.Render(fmt.Sprintf("%-20s", sourceName)),
			title,
		)

		if i >= listTop-1 {
			break
		}
	}

	return nil
}
