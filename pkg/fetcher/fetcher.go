package fetcher

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"
	"github.com/samueltuoyo15/Rss-Feed-Aggregator/pkg/models"
)

func FetchAll(feeds []models.Feed) ([]models.FeedWithItems, map[string][]models.FeedItem, error) {
	var (
		wg sync.WaitGroup
		mu sync.Mutex
		parser = gofeed.NewParser()
		client = &http.Client{Timeout: 10 * time.Second}
		results = make([]models.FeedWithItems, len(feeds))
		feedMap = make(map[string][]models.FeedItem)
	)

	for i, feed := range feeds {
		wg.Add(1)
		go func(i int, f models.Feed) {
			defer wg.Done()

			parsed, err := parser.ParseURL(f.Url)
			if err != nil {
				log.Printf("Failed to fetch %s: %v", f.Name, err)
				return
			}

			items := make([]models.FeedItem, len(parsed.Items))
			var itemWg sync.WaitGroup

			for idx, item := range parsed.Items {
				items[idx] = models.FeedItem{
					Title:       item.Title,
					Link:        item.Link,
					PublishedAt: item.Published,
					Description: item.Description,
					Categories:  item.Categories,
					Content:      item.Content,
					Author:       item.Author,
					GUID:         item.GUID,
				}

				thumb := extractThumbnail(item)
				if thumb != "" {
					items[idx].ThumbnailUrl = thumb
					continue
				}

				if strings.Contains(f.Url, "techcrunch.com") {
					itemWg.Add(1)
					go func(targetIdx int, link string) {
						defer itemWg.Done()
						items[targetIdx].ThumbnailUrl = getTechCrunchThumbnail(link, client)
					}(idx, item.Link)
				}
			}

			itemWg.Wait()

			mu.Lock()
			results[i] = models.FeedWithItems{
				Feed:  f,
				Items: items,
			}
			feedMap[f.Name] = items
			mu.Unlock()
		}(i, feed)
	}

	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		isTechCrunchI := strings.Contains(results[i].Feed.Url, "techcrunch.com")
		isTechCrunchJ := strings.Contains(results[j].Feed.Url, "techcrunch.com")
		if isTechCrunchI && !isTechCrunchJ {
			return true
		}
		if !isTechCrunchI && isTechCrunchJ {
			return false
		}
		return results[i].Feed.Name < results[j].Feed.Name
	})

	return results, feedMap, nil
}

func extractThumbnail(item *gofeed.Item) string {
	if mediaExt, ok := item.Extensions["media"]; ok {
		if thumbs, ok := mediaExt["thumbnail"]; ok && len(thumbs) > 0 {
			return thumbs[0].Attrs["url"]
		}
	}
	for _, enc := range item.Enclosures {
		if strings.HasPrefix(enc.Type, "image/") {
			return enc.URL
		}
	}
	return ""
}

func getTechCrunchThumbnail(url string, client *http.Client) string {
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return ""
	}
	if ogImage, exists := doc.Find("meta[property='og:image']").Attr("content"); exists {
		return ogImage
	}
	if twitterImage, exists := doc.Find("meta[name='twitter:image']").Attr("content"); exists {
		return twitterImage
	}
	if featuredImage, exists := doc.Find("img.featured-image").Attr("src"); exists {
		return featuredImage
	}
	return ""
}