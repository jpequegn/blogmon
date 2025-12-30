package graph

import (
	"sort"
	"time"
)

type Trend struct {
	Topic       string
	Count       int
	Score       float64
	RecentPosts []int64
}

type TrendAnalyzer struct {
	postTopics map[int64]topicEntry
}

type topicEntry struct {
	topics    []string
	timestamp time.Time
}

func NewTrendAnalyzer() *TrendAnalyzer {
	return &TrendAnalyzer{
		postTopics: make(map[int64]topicEntry),
	}
}

func (ta *TrendAnalyzer) AddPost(postID int64, topics []string, publishedAt time.Time) {
	ta.postTopics[postID] = topicEntry{
		topics:    topics,
		timestamp: publishedAt,
	}
}

func (ta *TrendAnalyzer) GetTrends(days int, limit int) []Trend {
	cutoff := time.Now().AddDate(0, 0, -days)

	// Count topics and track recency
	topicCounts := make(map[string]int)
	topicPosts := make(map[string][]int64)
	topicRecency := make(map[string]time.Time)

	for postID, entry := range ta.postTopics {
		for _, topic := range entry.topics {
			topicCounts[topic]++
			topicPosts[topic] = append(topicPosts[topic], postID)

			// Track most recent post for each topic
			if existing, ok := topicRecency[topic]; !ok || entry.timestamp.After(existing) {
				topicRecency[topic] = entry.timestamp
			}
		}
	}

	// Calculate trend scores
	var trends []Trend
	for topic, count := range topicCounts {
		// Count recent posts
		recentCount := 0
		var recentPosts []int64
		for _, postID := range topicPosts[topic] {
			if entry, ok := ta.postTopics[postID]; ok && entry.timestamp.After(cutoff) {
				recentCount++
				recentPosts = append(recentPosts, postID)
			}
		}

		// Score: recent count weighted higher than total count
		// Also factor in how recent the most recent post is
		recencyBoost := 1.0
		if mostRecent, ok := topicRecency[topic]; ok {
			daysSince := time.Since(mostRecent).Hours() / 24
			if daysSince < float64(days) {
				recencyBoost = 1.0 + (float64(days)-daysSince)/float64(days)
			}
		}

		score := (float64(recentCount)*2 + float64(count)) * recencyBoost

		trends = append(trends, Trend{
			Topic:       topic,
			Count:       count,
			Score:       score,
			RecentPosts: recentPosts,
		})
	}

	// Sort by score
	sort.Slice(trends, func(i, j int) bool {
		return trends[i].Score > trends[j].Score
	})

	if len(trends) > limit {
		trends = trends[:limit]
	}

	return trends
}
