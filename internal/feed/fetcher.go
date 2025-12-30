package feed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
)

type FetchedPost struct {
	URL         string
	Title       string
	Author      string
	PublishedAt time.Time
	Content     string
}

type Fetcher struct {
	parser *gofeed.Parser
	client *http.Client
}

func NewFetcher(timeout time.Duration) *Fetcher {
	return &Fetcher{
		parser: gofeed.NewParser(),
		client: &http.Client{Timeout: timeout},
	}
}

func (f *Fetcher) FetchFeed(feedURL string) ([]FetchedPost, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	feed, err := f.parser.ParseURLWithContext(feedURL, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse feed: %w", err)
	}

	var posts []FetchedPost
	for _, item := range feed.Items {
		post := FetchedPost{
			URL:   item.Link,
			Title: item.Title,
		}

		if item.Author != nil {
			post.Author = item.Author.Name
		} else if len(feed.Authors) > 0 {
			post.Author = feed.Authors[0].Name
		}

		if item.PublishedParsed != nil {
			post.PublishedAt = *item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			post.PublishedAt = *item.UpdatedParsed
		} else {
			post.PublishedAt = time.Now()
		}

		if item.Content != "" {
			post.Content = item.Content
		} else {
			post.Content = item.Description
		}

		posts = append(posts, post)
	}

	return posts, nil
}

func (f *Fetcher) FetchFullContent(postURL string) (string, error) {
	resp, err := f.client.Get(postURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return "", err
	}

	return string(body), nil
}
