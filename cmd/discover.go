package cmd

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/feed"
	"github.com/julienpequegnot/blogmon/internal/reference"
	"github.com/julienpequegnot/blogmon/internal/source"
	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover new blogs from references",
	Long:  `Analyzes blog references in posts to discover and optionally add new sources.`,
	RunE:  runDiscover,
}

var (
	discoverAutoAdd bool
	discoverLimit   int
)

func init() {
	rootCmd.AddCommand(discoverCmd)
	discoverCmd.Flags().BoolVar(&discoverAutoAdd, "auto-add", false, "Automatically add discovered blogs")
	discoverCmd.Flags().IntVarP(&discoverLimit, "limit", "l", 20, "Maximum blogs to discover")
}

func runDiscover(cmd *cobra.Command, args []string) error {
	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	refRepo := reference.NewRepository(db)
	srcRepo := source.NewRepository(db)

	// Get all blog references
	blogRefs, err := refRepo.ListBlogReferences()
	if err != nil {
		return err
	}

	if len(blogRefs) == 0 {
		fmt.Println("No blog references found. Run 'blogmon extract' first.")
		return nil
	}

	fmt.Printf("Analyzing %d blog references\n\n", len(blogRefs))

	// Group by domain and count
	domainCounts := make(map[string]int)
	domainPostIDs := make(map[string]int64) // Track which post referenced it
	domainURLs := make(map[string]string)

	for _, ref := range blogRefs {
		parsed, err := url.Parse(ref.URL)
		if err != nil {
			continue
		}

		domain := parsed.Host
		// Normalize domain
		domain = strings.TrimPrefix(domain, "www.")

		domainCounts[domain]++
		if _, exists := domainPostIDs[domain]; !exists {
			domainPostIDs[domain] = ref.PostID
			domainURLs[domain] = "https://" + domain
		}
	}

	// Sort by count and filter to limit
	type domainEntry struct {
		domain string
		count  int
	}
	var entries []domainEntry
	for domain, count := range domainCounts {
		entries = append(entries, domainEntry{domain, count})
	}

	// Simple bubble sort by count (descending)
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].count > entries[i].count {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	if len(entries) > discoverLimit {
		entries = entries[:discoverLimit]
	}

	discovered := 0
	added := 0

	for _, entry := range entries {
		domain := entry.domain
		count := entry.count
		refURL := domainURLs[domain]

		// Check if already a source
		if _, err := srcRepo.GetByURL(refURL); err == nil {
			continue // Already exists
		}

		fmt.Printf("Discovered: %s (referenced %d times)\n", domain, count)
		discovered++

		if discoverAutoAdd {
			// Try to find RSS feed
			feedURL, err := feed.DiscoverFeed(refURL)
			if err != nil || feedURL == "" {
				fmt.Printf("  → Could not find RSS feed\n")
				continue
			}

			// Add as discovered source
			postID := domainPostIDs[domain]
			_, err = srcRepo.AddDiscovered(refURL, domain, feedURL, postID)
			if err != nil {
				fmt.Printf("  → Error adding: %v\n", err)
				continue
			}

			fmt.Printf("  → Added with feed: %s\n", feedURL)
			added++
		}
	}

	fmt.Printf("\nDiscovered %d new blogs", discovered)
	if discoverAutoAdd {
		fmt.Printf(", added %d", added)
	}
	fmt.Println()

	if !discoverAutoAdd && discovered > 0 {
		fmt.Println("\nRun with --auto-add to automatically add discovered blogs.")
	}

	return nil
}
