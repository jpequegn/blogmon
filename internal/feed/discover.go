// internal/feed/discover.go
package feed

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var feedPatterns = []string{
	"/feed",
	"/feed.xml",
	"/atom.xml",
	"/rss.xml",
	"/rss",
	"/index.xml",
	"/feed/atom",
	"/feed/rss",
}

var linkRegex = regexp.MustCompile(`<link[^>]+type=["'](application/(rss|atom)\+xml)["'][^>]*href=["']([^"']+)["']`)

func DiscoverFeed(siteURL string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Try to find feed link in HTML
	resp, err := client.Get(siteURL)
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 100000))
		matches := linkRegex.FindStringSubmatch(string(body))
		if len(matches) > 3 {
			feedURL := matches[3]
			if !strings.HasPrefix(feedURL, "http") {
				feedURL = strings.TrimSuffix(siteURL, "/") + "/" + strings.TrimPrefix(feedURL, "/")
			}
			return feedURL, nil
		}
	}

	// Try common feed paths
	baseURL := strings.TrimSuffix(siteURL, "/")
	for _, pattern := range feedPatterns {
		feedURL := baseURL + pattern
		resp, err := client.Head(feedURL)
		if err == nil && resp.StatusCode == 200 {
			return feedURL, nil
		}
	}

	return "", fmt.Errorf("could not discover feed for %s", siteURL)
}
