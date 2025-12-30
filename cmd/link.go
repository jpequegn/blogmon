package cmd

import (
	"fmt"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/graph"
	"github.com/julienpequegnot/blogmon/internal/insight"
	"github.com/julienpequegnot/blogmon/internal/link"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/spf13/cobra"
)

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Build concept graph by linking related posts",
	Long:  `Analyzes posts and creates links between those sharing similar topics.`,
	RunE:  runLink,
}

var (
	linkMinSimilarity float64
	linkRebuild       bool
)

func init() {
	rootCmd.AddCommand(linkCmd)
	linkCmd.Flags().Float64Var(&linkMinSimilarity, "min-similarity", 0.3, "Minimum similarity threshold for linking")
	linkCmd.Flags().BoolVar(&linkRebuild, "rebuild", false, "Rebuild all links from scratch")
}

func runLink(cmd *cobra.Command, args []string) error {
	db, err := database.New(config.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	postRepo := post.NewRepository(db)
	insightRepo := insight.NewRepository(db)
	linkRepo := link.NewRepository(db)

	// Get all posts
	posts, err := postRepo.List(1000, 0)
	if err != nil {
		return err
	}

	if len(posts) < 2 {
		fmt.Println("Need at least 2 posts to build links.")
		return nil
	}

	fmt.Printf("Analyzing %d posts for topic similarity\n\n", len(posts))

	// Extract topics for each post
	postTopics := make(map[int64][]string)
	for _, p := range posts {
		// Get insights for this post
		insights, _ := insightRepo.ListForPost(p.ID)

		var insightTexts []string
		for _, ins := range insights {
			insightTexts = append(insightTexts, ins.Content)
		}

		// Extract topics from title + content + insights
		content := p.Title + " " + p.ContentClean
		if len(insightTexts) > 0 {
			content += " " + fmt.Sprintf("%v", insightTexts)
		}

		topics := graph.ExtractTopics(content)
		postTopics[p.ID] = topics

		if len(topics) > 0 {
			fmt.Printf("  %s: %v\n", truncateLinkTitle(p.Title, 40), topics)
		}
	}

	fmt.Println()

	// Compare all pairs and create links
	linksCreated := 0
	for i := 0; i < len(posts); i++ {
		for j := i + 1; j < len(posts); j++ {
			postA := posts[i]
			postB := posts[j]

			topicsA := postTopics[postA.ID]
			topicsB := postTopics[postB.ID]

			if len(topicsA) == 0 || len(topicsB) == 0 {
				continue
			}

			similarity := graph.ComputeTopicSimilarity(topicsA, topicsB)

			if similarity >= linkMinSimilarity {
				// Find shared topics for relationship description
				sharedTopics := findSharedTopics(topicsA, topicsB)
				relationship := "shared_topics:" + joinTopics(sharedTopics)

				if err := linkRepo.Upsert(postA.ID, postB.ID, relationship, similarity); err != nil {
					fmt.Printf("Error linking posts: %v\n", err)
					continue
				}

				fmt.Printf("Linked: '%s' <-> '%s' (%.2f)\n",
					truncateLinkTitle(postA.Title, 25),
					truncateLinkTitle(postB.Title, 25),
					similarity)
				linksCreated++
			}
		}
	}

	fmt.Printf("\nCreated %d links\n", linksCreated)
	return nil
}

func truncateLinkTitle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func findSharedTopics(a, b []string) []string {
	setB := make(map[string]bool)
	for _, t := range b {
		setB[t] = true
	}

	var shared []string
	for _, t := range a {
		if setB[t] {
			shared = append(shared, t)
		}
	}
	return shared
}

func joinTopics(topics []string) string {
	if len(topics) == 0 {
		return "general"
	}
	result := topics[0]
	for i := 1; i < len(topics) && i < 3; i++ {
		result += "," + topics[i]
	}
	return result
}
