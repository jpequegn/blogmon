package scorer

import (
	"testing"
)

func TestNoveltyScorer(t *testing.T) {
	scorer := NewNoveltyScorer()

	// Add some existing content
	scorer.AddDocument(1, "golang programming concurrency")
	scorer.AddDocument(2, "rust memory safety ownership")

	// Similar content should have low novelty
	similarScore := scorer.Score("golang programming goroutines concurrency")

	// Different content should have high novelty
	differentScore := scorer.Score("machine learning neural networks tensorflow")

	if similarScore >= differentScore {
		t.Errorf("expected similar score (%f) < different score (%f)", similarScore, differentScore)
	}
}

func TestNoveltyEmptyCorpus(t *testing.T) {
	scorer := NewNoveltyScorer()
	score := scorer.Score("any content here")

	if score != 100.0 {
		t.Errorf("expected 100 novelty for empty corpus, got %f", score)
	}
}
