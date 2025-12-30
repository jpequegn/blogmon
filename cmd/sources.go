// cmd/sources.go
package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/source"
	"github.com/spf13/cobra"
)

var sourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "List monitored sources",
	Long:  `Display all blogs and RSS feeds being monitored.`,
	RunE:  runSources,
}

func init() {
	rootCmd.AddCommand(sourcesCmd)
}

func runSources(cmd *cobra.Command, args []string) error {
	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	repo := source.NewRepository(db)
	sources, err := repo.List()
	if err != nil {
		return err
	}

	if len(sources) == 0 {
		fmt.Println("No sources configured. Add some with 'blogmon add <url>'")
		return nil
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	urlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14"))

	fmt.Println(headerStyle.Render(fmt.Sprintf(" %-4s  %-25s  %s", "ID", "NAME", "URL")))
	fmt.Println(strings.Repeat("─", 80))

	for _, s := range sources {
		name := s.Name
		if len(name) > 25 {
			name = name[:22] + "..."
		}

		fmt.Printf(" %s  %s  %s\n",
			idStyle.Render(fmt.Sprintf("%-4d", s.ID)),
			nameStyle.Render(fmt.Sprintf("%-25s", name)),
			urlStyle.Render(s.URL),
		)
	}

	return nil
}
