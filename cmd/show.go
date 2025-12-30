// cmd/show.go
package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <post-id>",
	Short: "Show details of a post",
	Long:  `Display full details of a post including content and metadata.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid post ID: %s", args[0])
	}

	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	repo := post.NewRepository(db)
	p, err := repo.Get(id)
	if err != nil {
		return fmt.Errorf("post not found: %d", id)
	}

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	urlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Underline(true)
	divider := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(strings.Repeat("━", 70))

	fmt.Println(divider)
	fmt.Println(titleStyle.Render(p.Title))
	fmt.Println(divider)

	fmt.Printf("%s %s\n", labelStyle.Render("Source:"), valueStyle.Render(p.SourceName))
	if p.Author != "" {
		fmt.Printf("%s %s\n", labelStyle.Render("Author:"), valueStyle.Render(p.Author))
	}
	if p.PublishedAt != nil {
		fmt.Printf("%s %s\n", labelStyle.Render("Published:"), valueStyle.Render(p.PublishedAt.Format("2006-01-02 15:04")))
	}
	if p.FinalScore != nil && *p.FinalScore > 0 {
		fmt.Printf("%s %.0f\n", labelStyle.Render("Score:"), *p.FinalScore)
	}
	fmt.Printf("%s %s\n", labelStyle.Render("URL:"), urlStyle.Render(p.URL))

	fmt.Println()

	// Show content preview
	content := p.ContentClean
	if content == "" {
		content = stripHTML(p.ContentRaw)
	}
	if len(content) > 500 {
		content = content[:500] + "..."
	}

	if content != "" {
		fmt.Println(labelStyle.Render("PREVIEW:"))
		fmt.Println(valueStyle.Render(content))
	}

	return nil
}

func stripHTML(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}
