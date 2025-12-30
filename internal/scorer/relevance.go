package scorer

import (
	"strings"

	"github.com/julienpequegnot/blogmon/internal/config"
)

type RelevanceScorer struct {
	interests []config.Interest
}

func NewRelevanceScorer(interests []config.Interest) *RelevanceScorer {
	return &RelevanceScorer{interests: interests}
}

func (s *RelevanceScorer) Score(title, content string) float64 {
	if len(s.interests) == 0 {
		return 50.0 // Neutral score if no interests configured
	}

	text := strings.ToLower(title + " " + content)
	wordCount := len(strings.Fields(text))
	if wordCount == 0 {
		return 0
	}

	var totalScore float64
	var totalWeight float64

	for _, interest := range s.interests {
		topic := strings.ToLower(interest.Topic)
		keywords := append([]string{topic}, interest.Keywords...)

		matchCount := 0
		for _, keyword := range keywords {
			matchCount += strings.Count(text, strings.ToLower(keyword))
		}

		// Normalize by word count and apply weight
		if matchCount > 0 {
			density := float64(matchCount) / float64(wordCount) * 1000
			totalScore += density * interest.Weight
		}
		totalWeight += interest.Weight
	}

	if totalWeight == 0 {
		return 50.0
	}

	// Normalize to 0-100 scale
	score := (totalScore / totalWeight) * 10
	if score > 100 {
		score = 100
	}

	return score
}
