package cmd

import (
	"fmt"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/julienpequegnot/blogmon/internal/post"
	"github.com/julienpequegnot/blogmon/internal/score"
	"github.com/julienpequegnot/blogmon/internal/scorer"
	"github.com/spf13/cobra"
)

var scoreCmd = &cobra.Command{
	Use:   "score",
	Short: "Calculate scores for posts",
	Long:  `Calculates community, relevance, and novelty scores for unscored posts.`,
	RunE:  runScore,
}

var (
	scoreLimit  int
	scoreSkipHN bool
)

func init() {
	rootCmd.AddCommand(scoreCmd)
	scoreCmd.Flags().IntVarP(&scoreLimit, "limit", "l", 50, "Maximum posts to score")
	scoreCmd.Flags().BoolVar(&scoreSkipHN, "skip-hn", false, "Skip HN API calls (for testing)")
}

func runScore(cmd *cobra.Command, args []string) error {
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
	scoreRepo := score.NewRepository(db)

	// Get unscored post IDs
	unscoredIDs, err := scoreRepo.GetUnscoredPostIDs(scoreLimit)
	if err != nil {
		return err
	}

	if len(unscoredIDs) == 0 {
		fmt.Println("No unscored posts found.")
		return nil
	}

	fmt.Printf("Scoring %d posts\n\n", len(unscoredIDs))

	// Initialize scorers
	hnScorer := scorer.NewHNScorer()
	relevanceScorer := scorer.NewRelevanceScorer(cfg.Interests)
	noveltyScorer := scorer.NewNoveltyScorer()

	// Build novelty corpus from existing posts
	allPosts, _ := postRepo.List(1000, 0)
	for _, p := range allPosts {
		content := p.ContentClean
		if content == "" {
			content = p.Title
		}
		noveltyScorer.AddDocument(p.ID, content)
	}

	weights := cfg.Scoring

	for _, postID := range unscoredIDs {
		p, err := postRepo.Get(postID)
		if err != nil {
			fmt.Printf("Error getting post %d: %v\n", postID, err)
			continue
		}

		fmt.Printf("Scoring: %s\n", p.Title)

		// Community score (HN)
		var communityScore float64
		if !scoreSkipHN {
			hit, err := hnScorer.SearchByURL(p.URL)
			if err != nil {
				fmt.Printf("  HN API error: %v\n", err)
			} else if hit != nil {
				communityScore = scorer.CalculateCommunityScore(hit.Points, hit.NumComments, 0)
				fmt.Printf("  HN: %d points, %d comments\n", hit.Points, hit.NumComments)
			}
		}

		// Relevance score
		content := p.ContentClean
		if content == "" {
			content = p.Title
		}
		relevanceScore := relevanceScorer.Score(p.Title, content)

		// Novelty score
		noveltyScore := noveltyScorer.Score(content)

		// Final score
		finalScore := (communityScore * weights.Community) +
			(relevanceScore * weights.Relevance) +
			(noveltyScore * weights.Novelty)

		// Save score
		if err := scoreRepo.Upsert(postID, communityScore, relevanceScore, noveltyScore, finalScore); err != nil {
			fmt.Printf("  Error saving score: %v\n", err)
			continue
		}

		fmt.Printf("  Community: %.1f, Relevance: %.1f, Novelty: %.1f -> Final: %.1f\n",
			communityScore, relevanceScore, noveltyScore, finalScore)
	}

	fmt.Println("\nScoring complete")
	return nil
}
