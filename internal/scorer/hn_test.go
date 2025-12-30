package scorer

import (
	"testing"
)

func TestCalculateCommunityScore(t *testing.T) {
	// Test score calculation formula
	score := CalculateCommunityScore(100, 50, 0)

	if score <= 0 {
		t.Error("expected positive score")
	}
	if score > 100 {
		t.Error("expected score <= 100")
	}
}

func TestCalculateCommunityScoreZero(t *testing.T) {
	score := CalculateCommunityScore(0, 0, 0)

	if score != 0 {
		t.Errorf("expected 0 score for no signals, got %f", score)
	}
}
