package cache

import (
	"sync"
	"time"

	"github.com/samueltuoyo15/Rss-Feed-Aggregator/pkg/models"
)

type FeedCache struct {
	mu        sync.RWMutex
	feeds     []models.FeedWithItems
	updatedAt time.Time
	ttl       time.Duration
}

func NewFeedCache(ttl time.Duration) *FeedCache {
	return &FeedCache{
		ttl: ttl,
	}
}

func (c *FeedCache) Get() ([]models.FeedWithItems, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.feeds) == 0 {
		return nil, false
	}

	if time.Since(c.updatedAt) > c.ttl {
		return nil, false
	}

	return c.feeds, true
}

func (c *FeedCache) Set(feeds []models.FeedWithItems) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.feeds = feeds
	c.updatedAt = time.Now()
}

func (c *FeedCache) GetOrFetch(fetchFn func() ([]models.FeedWithItems, error)) ([]models.FeedWithItems, error) {
	if feeds, found := c.Get(); found {
		return feeds, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.feeds) > 0 && time.Since(c.updatedAt) <= c.ttl {
		return c.feeds, nil
	}

	feeds, err := fetchFn()
	if err != nil {
		if len(c.feeds) > 0 {
			return c.feeds, nil
		}
		return nil, err
	}

	c.feeds = feeds
	c.updatedAt = time.Now()
	return feeds, nil
}
