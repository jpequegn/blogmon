package scorer

import (
	"math"
	"strings"
)

type NoveltyScorer struct {
	documents    map[int64]map[string]float64 // postID -> term -> tf
	documentFreq map[string]int               // term -> doc count
	docCount     int
}

func NewNoveltyScorer() *NoveltyScorer {
	return &NoveltyScorer{
		documents:    make(map[int64]map[string]float64),
		documentFreq: make(map[string]int),
		docCount:     0,
	}
}

func (s *NoveltyScorer) AddDocument(id int64, content string) {
	terms := tokenize(content)
	if len(terms) == 0 {
		return
	}

	tf := make(map[string]float64)
	termCounts := make(map[string]int)

	for _, term := range terms {
		termCounts[term]++
	}

	// Calculate term frequency
	for term, count := range termCounts {
		tf[term] = float64(count) / float64(len(terms))
	}

	// Update document frequency (only count each term once per doc)
	for term := range termCounts {
		s.documentFreq[term]++
	}

	s.documents[id] = tf
	s.docCount++
}

func (s *NoveltyScorer) Score(content string) float64 {
	if s.docCount == 0 {
		return 100.0 // Everything is novel when corpus is empty
	}

	terms := tokenize(content)
	if len(terms) == 0 {
		return 100.0
	}

	// Calculate TF-IDF vector for new content
	newTF := make(map[string]float64)
	termCounts := make(map[string]int)

	for _, term := range terms {
		termCounts[term]++
	}

	for term, count := range termCounts {
		newTF[term] = float64(count) / float64(len(terms))
	}

	// Find max cosine similarity with existing documents
	maxSimilarity := 0.0
	for _, docTF := range s.documents {
		similarity := s.cosineSimilarity(newTF, docTF)
		if similarity > maxSimilarity {
			maxSimilarity = similarity
		}
	}

	// Novelty = 1 - max_similarity, scaled to 0-100
	novelty := (1 - maxSimilarity) * 100
	return novelty
}

func (s *NoveltyScorer) cosineSimilarity(tf1, tf2 map[string]float64) float64 {
	var dotProduct, norm1, norm2 float64

	for term, tfidf1 := range tf1 {
		idf := s.idf(term)
		v1 := tfidf1 * idf
		norm1 += v1 * v1

		if tfidf2, ok := tf2[term]; ok {
			v2 := tfidf2 * idf
			dotProduct += v1 * v2
		}
	}

	for term, tfidf2 := range tf2 {
		idf := s.idf(term)
		v2 := tfidf2 * idf
		norm2 += v2 * v2
	}

	if norm1 == 0 || norm2 == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
}

func (s *NoveltyScorer) idf(term string) float64 {
	df := s.documentFreq[term]
	if df == 0 {
		return 0
	}
	return math.Log(float64(s.docCount+1) / float64(df+1))
}

func tokenize(content string) []string {
	content = strings.ToLower(content)

	// Simple tokenization: split on non-alphanumeric
	var tokens []string
	var current strings.Builder

	for _, r := range content {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			token := current.String()
			if len(token) > 2 { // Skip very short tokens
				tokens = append(tokens, token)
			}
			current.Reset()
		}
	}

	if current.Len() > 2 {
		tokens = append(tokens, current.String())
	}

	return tokens
}
