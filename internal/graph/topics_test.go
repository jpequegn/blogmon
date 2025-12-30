package graph

import (
	"testing"
)

func TestExtractTopics(t *testing.T) {
	content := "This article discusses golang concurrency patterns and goroutines for distributed systems"

	topics := ExtractTopics(content)

	if len(topics) == 0 {
		t.Error("expected topics to be extracted")
	}

	// Should find "golang" as a topic
	found := false
	for _, topic := range topics {
		if topic == "golang" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'golang' to be extracted as a topic")
	}
}

func TestComputeTopicSimilarity(t *testing.T) {
	topics1 := []string{"golang", "concurrency", "performance"}
	topics2 := []string{"golang", "performance", "rust"}

	similarity := ComputeTopicSimilarity(topics1, topics2)

	if similarity <= 0 {
		t.Error("expected positive similarity for overlapping topics")
	}
	if similarity > 1.0 {
		t.Error("expected similarity <= 1.0")
	}
}

func TestComputeTopicSimilarityNoOverlap(t *testing.T) {
	topics1 := []string{"golang", "concurrency"}
	topics2 := []string{"python", "machine-learning"}

	similarity := ComputeTopicSimilarity(topics1, topics2)

	if similarity != 0 {
		t.Errorf("expected 0 similarity for non-overlapping topics, got %f", similarity)
	}
}
