// cmd/daemon.go
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/feed"
	"github.com/julienpequegnot/blogmon/internal/graph"
	"github.com/julienpequegnot/blogmon/internal/insight"
	"github.com/julienpequegnot/blogmon/internal/link"
	"github.com/julienpequegnot/blogmon/internal/llm"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/julienpequegnot/blogmon/internal/reference"
	"github.com/julienpequegnot/blogmon/internal/score"
	"github.com/julienpequegnot/blogmon/internal/scorer"
	"github.com/julienpequegnot/blogmon/internal/source"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run in daemon mode",
	Long:  `Runs blogmon in the background, periodically fetching and processing new posts.`,
	RunE:  runDaemon,
}

var (
	daemonInterval int
	daemonOnce     bool
)

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.Flags().IntVar(&daemonInterval, "interval", 0, "Override interval in hours (0 = use config)")
	daemonCmd.Flags().BoolVar(&daemonOnce, "once", false, "Run once and exit")
}

func runDaemon(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	interval := cfg.Daemon.IntervalHours
	if daemonInterval > 0 {
		interval = daemonInterval
	}

	fmt.Printf("Blogmon daemon starting (interval: %d hours)\n", interval)

	// Run immediately on start
	if err := runPipeline(cfg); err != nil {
		fmt.Printf("Pipeline error: %v\n", err)
	}

	if daemonOnce {
		fmt.Println("Single run complete.")
		return nil
	}

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(interval) * time.Hour)
	defer ticker.Stop()

	fmt.Printf("Daemon running. Next run in %d hours. Press Ctrl+C to stop.\n", interval)

	for {
		select {
		case <-ticker.C:
			fmt.Printf("\n[%s] Running scheduled pipeline...\n", time.Now().Format("2006-01-02 15:04:05"))
			if err := runPipeline(cfg); err != nil {
				fmt.Printf("Pipeline error: %v\n", err)
			}
			fmt.Printf("Next run in %d hours.\n", interval)

		case sig := <-sigChan:
			fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
			cancel()
			return nil

		case <-ctx.Done():
			return nil
		}
	}
}

func runPipeline(cfg *config.Config) error {
	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	// Stage 1: Fetch
	fmt.Println("→ Fetching new posts...")
	srcRepo := source.NewRepository(db)
	postRepo := post.NewRepository(db)

	sources, err := srcRepo.List()
	if err != nil {
		return err
	}

	fetcher := feed.NewFetcher(time.Duration(cfg.Fetch.TimeoutSeconds) * time.Second)
	newPosts := 0

	for _, src := range sources {
		if src.FeedURL == "" {
			continue
		}

		items, err := fetcher.FetchFeed(src.FeedURL)
		if err != nil {
			fmt.Printf("  Error fetching %s: %v\n", src.Name, err)
			continue
		}

		for _, item := range items {
			exists, _ := postRepo.Exists(item.URL)
			if exists {
				continue
			}

			_, err := postRepo.Add(src.ID, item.URL, item.Title, item.Author, item.PublishedAt, item.Content)
			if err == nil {
				newPosts++
			}
		}

		srcRepo.UpdateLastFetched(src.ID)
	}
	fmt.Printf("  Fetched %d new posts\n", newPosts)

	if newPosts == 0 {
		fmt.Println("→ No new posts to process")
		return nil
	}

	// Stage 2: Extract (limit to new posts)
	fmt.Println("→ Extracting insights...")
	insightRepo := insight.NewRepository(db)
	refRepo := reference.NewRepository(db)

	llmClient := llm.NewClient("http://localhost:11434", cfg.APIs.LLMModel, 2*time.Minute)

	unextracted, _ := postRepo.GetUnextracted(newPosts)
	extracted := 0
	for _, p := range unextracted {
		content := stripHTMLTags(p.ContentRaw)
		if len(content) < 100 {
			continue
		}
		if len(content) > 8000 {
			content = content[:8000]
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		result, err := llmClient.ExtractInsights(ctx, p.Title, content)
		cancel()

		if err != nil {
			continue
		}

		for i, takeaway := range result.Takeaways {
			importance := 5 - i
			if importance < 1 {
				importance = 1
			}
			insightRepo.Add(p.ID, "takeaway", takeaway, importance)
		}

		for _, ref := range result.References {
			isBlog := isBlogURL(ref.URL)
			refRepo.Add(p.ID, ref.URL, ref.Title, ref.Context, isBlog)
		}

		wordCount := len(content) / 5 // rough estimate
		postRepo.UpdateContentClean(p.ID, content, wordCount)
		extracted++
	}
	fmt.Printf("  Extracted insights from %d posts\n", extracted)

	// Stage 3: Score
	fmt.Println("→ Scoring posts...")
	scoreRepo := score.NewRepository(db)
	hnScorer := scorer.NewHNScorer()
	relevanceScorer := scorer.NewRelevanceScorer(cfg.Interests)
	noveltyScorer := scorer.NewNoveltyScorer()

	allPosts, _ := postRepo.List(1000, 0)
	for _, p := range allPosts {
		content := p.ContentClean
		if content == "" {
			content = p.Title
		}
		noveltyScorer.AddDocument(p.ID, content)
	}

	unscoredIDs, _ := scoreRepo.GetUnscoredPostIDs(newPosts)
	scored := 0
	for _, postID := range unscoredIDs {
		p, err := postRepo.Get(postID)
		if err != nil {
			continue
		}

		var communityScore float64
		hit, _ := hnScorer.SearchByURL(p.URL)
		if hit != nil {
			communityScore = scorer.CalculateCommunityScore(hit.Points, hit.NumComments, 0)
		}

		content := p.ContentClean
		if content == "" {
			content = p.Title
		}
		relevanceScore := relevanceScorer.Score(p.Title, content)
		noveltyScore := noveltyScorer.Score(content)

		finalScore := (communityScore * cfg.Scoring.Community) +
			(relevanceScore * cfg.Scoring.Relevance) +
			(noveltyScore * cfg.Scoring.Novelty)

		scoreRepo.Upsert(postID, communityScore, relevanceScore, noveltyScore, finalScore)
		scored++
	}
	fmt.Printf("  Scored %d posts\n", scored)

	// Stage 4: Link
	fmt.Println("→ Updating concept graph...")
	linkRepo := link.NewRepository(db)

	postTopics := make(map[int64][]string)
	for _, p := range allPosts {
		content := p.Title + " " + p.ContentClean
		topics := graph.ExtractTopics(content)
		postTopics[p.ID] = topics
	}

	linked := 0
	for i := 0; i < len(allPosts); i++ {
		for j := i + 1; j < len(allPosts); j++ {
			topicsA := postTopics[allPosts[i].ID]
			topicsB := postTopics[allPosts[j].ID]

			if len(topicsA) == 0 || len(topicsB) == 0 {
				continue
			}

			similarity := graph.ComputeTopicSimilarity(topicsA, topicsB)
			if similarity >= 0.3 {
				sharedTopics := findSharedTopics(topicsA, topicsB)
				relationship := "shared_topics:" + joinTopics(sharedTopics)
				if err := linkRepo.Upsert(allPosts[i].ID, allPosts[j].ID, relationship, similarity); err == nil {
					linked++
				}
			}
		}
	}
	fmt.Printf("  Created/updated %d links\n", linked)

	fmt.Println("→ Pipeline complete")
	return nil
}
