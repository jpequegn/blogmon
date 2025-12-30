package cmd

import (
	"fmt"
	"sync"
	"time"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/feed"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/julienpequegnot/blogmon/internal/source"
	"github.com/spf13/cobra"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch new posts from all monitored blogs",
	Long:  `Downloads new posts from RSS feeds of all active sources.`,
	RunE:  runFetch,
}

var fetchConcurrency int

func init() {
	rootCmd.AddCommand(fetchCmd)
	fetchCmd.Flags().IntVarP(&fetchConcurrency, "concurrency", "c", 5, "Number of concurrent fetches")
}

func runFetch(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	srcRepo := source.NewRepository(db)
	postRepo := post.NewRepository(db)

	sources, err := srcRepo.List()
	if err != nil {
		return err
	}

	if len(sources) == 0 {
		fmt.Println("No sources configured. Add some with 'blogmon add <url>'")
		return nil
	}

	fetcher := feed.NewFetcher(time.Duration(cfg.Fetch.TimeoutSeconds) * time.Second)

	var wg sync.WaitGroup
	sem := make(chan struct{}, fetchConcurrency)
	var mu sync.Mutex
	totalNew := 0

	for _, src := range sources {
		if src.FeedURL == "" {
			fmt.Printf("Skipping %s (no feed URL)\n", src.Name)
			continue
		}

		wg.Add(1)
		go func(s source.Source) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			fmt.Printf("Fetching %s...\n", s.Name)

			posts, err := fetcher.FetchFeed(s.FeedURL)
			if err != nil {
				fmt.Printf("  Error: %v\n", err)
				return
			}

			newCount := 0
			for _, p := range posts {
				exists, _ := postRepo.Exists(p.URL)
				if exists {
					continue
				}

				_, err := postRepo.Add(s.ID, p.URL, p.Title, p.Author, p.PublishedAt, p.Content)
				if err != nil {
					fmt.Printf("  Failed to save: %s\n", p.Title)
					continue
				}
				newCount++
			}

			srcRepo.UpdateLastFetched(s.ID)

			mu.Lock()
			totalNew += newCount
			mu.Unlock()

			fmt.Printf("  %s: %d new posts\n", s.Name, newCount)
		}(src)
	}

	wg.Wait()

	fmt.Printf("\nTotal: %d new posts fetched\n", totalNew)
	return nil
}
