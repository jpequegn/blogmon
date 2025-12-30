package cmd

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/insight"
	"github.com/julienpequegnot/blogmon/internal/llm"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/julienpequegnot/blogmon/internal/reference"
	"github.com/spf13/cobra"
)

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract insights from posts using LLM",
	Long:  `Analyzes unprocessed posts and extracts key takeaways, references, and topics.`,
	RunE:  runExtract,
}

var (
	extractLimit      int
	extractSkipErrors bool
)

func init() {
	rootCmd.AddCommand(extractCmd)
	extractCmd.Flags().IntVarP(&extractLimit, "limit", "l", 10, "Maximum posts to process")
	extractCmd.Flags().BoolVar(&extractSkipErrors, "skip-errors", false, "Continue on extraction errors")
}

func runExtract(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	postRepo := post.NewRepository(db)
	insightRepo := insight.NewRepository(db)
	refRepo := reference.NewRepository(db)

	// Get unextracted posts
	posts, err := postRepo.GetUnextracted(extractLimit)
	if err != nil {
		return err
	}

	if len(posts) == 0 {
		fmt.Println("No unprocessed posts found.")
		return nil
	}

	fmt.Printf("Found %d posts to process\n\n", len(posts))

	// Initialize LLM client
	llmClient := llm.NewClient(
		"http://localhost:11434",
		cfg.APIs.LLMModel,
		2*time.Minute,
	)

	processed := 0
	for _, p := range posts {
		fmt.Printf("Extracting: %s\n", p.Title)

		// Clean HTML content
		cleanContent := stripHTMLTags(p.ContentRaw)
		if len(cleanContent) < 100 {
			fmt.Printf("  Skipping (content too short)\n")
			continue
		}

		// Truncate for LLM context
		if len(cleanContent) > 8000 {
			cleanContent = cleanContent[:8000]
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		result, err := llmClient.ExtractInsights(ctx, p.Title, cleanContent)
		cancel()

		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			if !extractSkipErrors {
				return err
			}
			continue
		}

		// Save insights
		for i, takeaway := range result.Takeaways {
			importance := 5 - i // First takeaway most important
			if importance < 1 {
				importance = 1
			}
			insightRepo.Add(p.ID, "takeaway", takeaway, importance)
		}

		// Save references
		for _, ref := range result.References {
			isBlog := isBlogURL(ref.URL)
			refRepo.Add(p.ID, ref.URL, ref.Title, ref.Context, isBlog)
		}

		// Update post with clean content
		wordCount := len(strings.Fields(cleanContent))
		postRepo.UpdateContentClean(p.ID, cleanContent, wordCount)

		fmt.Printf("  Extracted %d takeaways, %d references\n", len(result.Takeaways), len(result.References))
		processed++
	}

	fmt.Printf("\nProcessed %d posts\n", processed)
	return nil
}

func stripHTMLTags(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	clean := re.ReplaceAllString(s, " ")
	clean = strings.Join(strings.Fields(clean), " ")
	return strings.TrimSpace(clean)
}

func isBlogURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	// Common blog indicators
	blogPatterns := []string{
		"blog", "post", "article", "writing",
	}

	host := strings.ToLower(parsed.Host)
	path := strings.ToLower(parsed.Path)

	for _, pattern := range blogPatterns {
		if strings.Contains(host, pattern) || strings.Contains(path, pattern) {
			return true
		}
	}

	// Known blog platforms
	blogDomains := []string{
		"medium.com", "dev.to", "hashnode", "substack.com",
	}

	for _, domain := range blogDomains {
		if strings.Contains(host, domain) {
			return true
		}
	}

	return false
}
