package scorer

import (
	"testing"

	"github.com/julienpequegnot/blogmon/internal/config"
)

func TestRelevanceScorer(t *testing.T) {
	interests := []config.Interest{
		{Topic: "golang", Weight: 1.0, Keywords: []string{"go", "goroutine"}},
		{Topic: "rust", Weight: 0.8},
	}

	scorer := NewRelevanceScorer(interests)

	// Post about Go should score high
	score := scorer.Score("Learning Go", "This is about golang and goroutines and go programming")
	if score <= 0 {
		t.Errorf("expected positive score for Go content, got %f", score)
	}

	// Unrelated post should score low
	lowScore := scorer.Score("Cooking Tips", "How to make pasta and pizza")
	if lowScore >= score {
		t.Errorf("expected cooking score (%f) < Go score (%f)", lowScore, score)
	}
}

func TestRelevanceScorerNoInterests(t *testing.T) {
	scorer := NewRelevanceScorer([]config.Interest{})
	score := scorer.Score("Any Title", "Any content")

	if score != 50.0 {
		t.Errorf("expected neutral score 50, got %f", score)
	}
}
