package graph

import (
	"regexp"
	"strings"
)

// Common tech topics to detect
var techTopics = map[string][]string{
	"golang":              {"go", "golang", "goroutine", "goroutines"},
	"rust":                {"rust", "rustlang", "cargo", "ownership"},
	"python":              {"python", "django", "flask", "pytorch"},
	"javascript":          {"javascript", "typescript", "nodejs", "react", "vue"},
	"distributed-systems": {"distributed", "consensus", "raft", "paxos", "microservices"},
	"databases":           {"database", "sql", "postgresql", "mysql", "redis", "mongodb"},
	"kubernetes":          {"kubernetes", "k8s", "docker", "containers", "helm"},
	"performance":         {"performance", "optimization", "latency", "throughput", "benchmark"},
	"security":            {"security", "authentication", "encryption", "vulnerability"},
	"machine-learning":    {"machine learning", "ml", "neural", "tensorflow", "pytorch"},
	"devops":              {"devops", "ci/cd", "jenkins", "github actions", "terraform"},
	"architecture":        {"architecture", "design patterns", "solid", "clean architecture"},
	"testing":             {"testing", "unit test", "integration test", "tdd"},
	"concurrency":         {"concurrency", "parallel", "async", "threads", "mutex"},
	"api":                 {"api", "rest", "graphql", "grpc", "openapi"},
}

// ExtractTopics identifies tech topics from content
func ExtractTopics(content string) []string {
	content = strings.ToLower(content)
	var found []string

	for topic, keywords := range techTopics {
		for _, keyword := range keywords {
			if strings.Contains(content, keyword) {
				found = append(found, topic)
				break
			}
		}
	}

	return found
}

// ExtractTopicsFromInsights extracts topics from insight strings
func ExtractTopicsFromInsights(insights []string) []string {
	combined := strings.Join(insights, " ")
	return ExtractTopics(combined)
}

// ComputeTopicSimilarity calculates Jaccard similarity between topic sets
func ComputeTopicSimilarity(topics1, topics2 []string) float64 {
	if len(topics1) == 0 || len(topics2) == 0 {
		return 0
	}

	set1 := make(map[string]bool)
	for _, t := range topics1 {
		set1[t] = true
	}

	set2 := make(map[string]bool)
	for _, t := range topics2 {
		set2[t] = true
	}

	// Jaccard similarity: intersection / union
	intersection := 0
	for t := range set1 {
		if set2[t] {
			intersection++
		}
	}

	union := len(set1)
	for t := range set2 {
		if !set1[t] {
			union++
		}
	}

	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// ExtractKeywords extracts significant words from content
func ExtractKeywords(content string, minLen int) []string {
	content = strings.ToLower(content)

	// Remove common stop words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"is": true, "are": true, "was": true, "were": true, "be": true,
		"been": true, "being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true, "must": true,
		"this": true, "that": true, "these": true, "those": true,
		"i": true, "you": true, "he": true, "she": true, "it": true,
		"we": true, "they": true, "what": true, "which": true, "who": true,
		"when": true, "where": true, "why": true, "how": true,
		"all": true, "each": true, "every": true, "both": true, "few": true,
		"more": true, "most": true, "other": true, "some": true, "such": true,
		"no": true, "not": true, "only": true, "same": true, "so": true,
		"than": true, "too": true, "very": true, "just": true, "also": true,
	}

	// Extract words
	re := regexp.MustCompile(`[a-z]+`)
	words := re.FindAllString(content, -1)

	seen := make(map[string]bool)
	var keywords []string

	for _, word := range words {
		if len(word) >= minLen && !stopWords[word] && !seen[word] {
			seen[word] = true
			keywords = append(keywords, word)
		}
	}

	return keywords
}
