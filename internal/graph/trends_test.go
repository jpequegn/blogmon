package graph

import (
	"testing"
	"time"
)

func TestTrendAnalyzer(t *testing.T) {
	analyzer := NewTrendAnalyzer()

	now := time.Now()

	// Add recent posts about golang
	analyzer.AddPost(1, []string{"golang", "performance"}, now)
	analyzer.AddPost(2, []string{"golang", "concurrency"}, now.AddDate(0, 0, -1))
	analyzer.AddPost(3, []string{"golang"}, now.AddDate(0, 0, -2))

	// Add older posts about rust
	analyzer.AddPost(4, []string{"rust", "performance"}, now.AddDate(0, -1, 0))
	analyzer.AddPost(5, []string{"rust"}, now.AddDate(0, -1, 0))

	trends := analyzer.GetTrends(7, 5)

	if len(trends) == 0 {
		t.Error("expected trends to be returned")
	}

	// Golang should be trending (3 recent posts)
	if trends[0].Topic != "golang" {
		t.Errorf("expected 'golang' to be top trend, got '%s'", trends[0].Topic)
	}
}

func TestTrendScore(t *testing.T) {
	analyzer := NewTrendAnalyzer()

	now := time.Now()

	// Add posts
	analyzer.AddPost(1, []string{"trending"}, now)
	analyzer.AddPost(2, []string{"old"}, now.AddDate(0, -2, 0))

	trends := analyzer.GetTrends(7, 10)

	// "trending" should have higher score than "old"
	var trendingScore, oldScore float64
	for _, tr := range trends {
		if tr.Topic == "trending" {
			trendingScore = tr.Score
		}
		if tr.Topic == "old" {
			oldScore = tr.Score
		}
	}

	if trendingScore <= oldScore {
		t.Errorf("expected trending score (%f) > old score (%f)", trendingScore, oldScore)
	}
}
