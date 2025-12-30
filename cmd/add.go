// cmd/add.go
package cmd

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/feed"
	"github.com/julienpequegnot/blogmon/internal/source"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <url>",
	Short: "Add a blog or RSS feed to monitor",
	Long:  `Add a blog URL or RSS feed URL to the list of monitored sources.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runAdd,
}

var addName string

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&addName, "name", "n", "", "Custom name for the blog")
}

func runAdd(cmd *cobra.Command, args []string) error {
	siteURL := args[0]

	// Ensure URL has scheme
	if !strings.HasPrefix(siteURL, "http") {
		siteURL = "https://" + siteURL
	}

	// Parse URL to extract domain for name
	parsed, err := url.Parse(siteURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	name := addName
	if name == "" {
		name = parsed.Host
	}

	// Try to discover RSS feed
	fmt.Printf("Discovering feed for %s...\n", siteURL)
	feedURL, err := feed.DiscoverFeed(siteURL)
	if err != nil {
		fmt.Printf("Warning: %v\n", err)
		fmt.Println("Adding without feed URL - you may need to add it manually")
	} else {
		fmt.Printf("Found feed: %s\n", feedURL)
	}

	// Open database
	db, err := database.New(config.DBPath())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Add source
	repo := source.NewRepository(db)
	src, err := repo.Add(siteURL, name, feedURL)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return fmt.Errorf("source already exists: %s", siteURL)
		}
		return err
	}

	fmt.Printf("\nAdded: %s (ID: %d)\n", src.Name, src.ID)
	fmt.Println("\nRun 'blogmon fetch' to download posts")

	return nil
}
