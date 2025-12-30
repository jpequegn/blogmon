package cmd

import (
	"fmt"
	"time"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/graph"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var trendsCmd = &cobra.Command{
	Use:   "trends",
	Short: "Show trending topics",
	Long:  `Analyzes posts to identify trending topics based on recency and frequency.`,
	RunE:  runTrends,
}

var (
	trendsDays  int
	trendsLimit int
)

func init() {
	rootCmd.AddCommand(trendsCmd)
	trendsCmd.Flags().IntVar(&trendsDays, "days", 30, "Time window in days")
	trendsCmd.Flags().IntVarP(&trendsLimit, "limit", "l", 10, "Maximum trends to show")
}

func runTrends(cmd *cobra.Command, args []string) error {
	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	postRepo := post.NewRepository(db)

	// Get all posts
	posts, err := postRepo.List(1000, 0)
	if err != nil {
		return err
	}

	if len(posts) == 0 {
		fmt.Println("No posts found.")
		return nil
	}

	// Build trend analyzer
	analyzer := graph.NewTrendAnalyzer()

	for _, p := range posts {
		content := p.Title + " " + p.ContentClean
		topics := graph.ExtractTopics(content)
		// Handle nil PublishedAt
		publishedAt := time.Now()
		if p.PublishedAt != nil {
			publishedAt = *p.PublishedAt
		}
		analyzer.AddPost(p.ID, topics, publishedAt)
	}

	trends := analyzer.GetTrends(trendsDays, trendsLimit)

	if len(trends) == 0 {
		fmt.Println("No trending topics found.")
		return nil
	}

	// Display trends
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	fmt.Printf("\n%s (last %d days)\n\n", titleStyle.Render("TRENDING TOPICS"), trendsDays)

	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	maxScore := trends[0].Score

	for i, trend := range trends {
		// Create bar visualization
		barWidth := int((trend.Score / maxScore) * 20)
		bar := ""
		for j := 0; j < barWidth; j++ {
			bar += "█"
		}

		fmt.Printf("%2d. %-20s %s %.1f (%d posts, %d recent)\n",
			i+1,
			trend.Topic,
			barStyle.Render(bar),
			trend.Score,
			trend.Count,
			len(trend.RecentPosts))
	}

	fmt.Println()
	return nil
}
