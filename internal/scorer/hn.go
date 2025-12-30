package scorer

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"time"
)

type HNSearchResponse struct {
	Hits []HNHit `json:"hits"`
}

type HNHit struct {
	ObjectID    string `json:"objectID"`
	Title       string `json:"title"`
	Points      int    `json:"points"`
	NumComments int    `json:"num_comments"`
	URL         string `json:"url"`
}

type HNScorer struct {
	client *http.Client
}

func NewHNScorer() *HNScorer {
	return &HNScorer{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *HNScorer) SearchByURL(postURL string) (*HNHit, error) {
	// HN Algolia API
	searchURL := fmt.Sprintf(
		"https://hn.algolia.com/api/v1/search?query=%s&restrictSearchableAttributes=url",
		url.QueryEscape(postURL),
	)

	resp, err := s.client.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to query HN API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HN API returned %d", resp.StatusCode)
	}

	var result HNSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode HN response: %w", err)
	}

	if len(result.Hits) == 0 {
		return nil, nil // Not found on HN
	}

	// Return the hit with the highest score
	var best *HNHit
	for i := range result.Hits {
		if best == nil || result.Hits[i].Points > best.Points {
			best = &result.Hits[i]
		}
	}

	return best, nil
}

func CalculateCommunityScore(hnPoints, hnComments, redditScore int) float64 {
	if hnPoints == 0 && hnComments == 0 && redditScore == 0 {
		return 0
	}

	// Weighted log scale to prevent viral posts from dominating
	// Formula: log(1 + hn_points*2 + hn_comments*3 + reddit_score) * 10
	rawScore := float64(hnPoints*2 + hnComments*3 + redditScore)
	score := math.Log(1+rawScore) * 10

	// Cap at 100
	if score > 100 {
		score = 100
	}

	return score
}
