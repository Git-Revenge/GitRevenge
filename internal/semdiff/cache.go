package semdiff

import (
	"container/list"
	"context"
	"crypto/sha256"
	"sync"
)

// Cache keeps recent results, so going back to a file, or switching between unified and
// split, doesn't compare it again. Texts are keyed by their SHA-256: a blob's id names its
// stored form, while the views compare the checked-out form, and the work tree has no id.
type Cache struct {
	mu      sync.Mutex
	size    int
	order   *list.List // of *entry, most recent first
	entries map[cacheKey]*list.Element
}

type cacheKey struct {
	before, after [sha256.Size]byte
	opts          Options
}

type entry struct {
	key cacheKey
	res Result
}

// NewCache returns a cache of the size most recent results.
func NewCache(size int) *Cache {
	return &Cache{size: size, order: list.New(), entries: map[cacheKey]*list.Element{}}
}

// Diff is Diff, from the cache when it can be.
func (c *Cache) Diff(ctx context.Context, before, after string, opts Options) (Result, error) {
	k := cacheKey{before: sha256.Sum256([]byte(before)), after: sha256.Sum256([]byte(after)), opts: opts}
	c.mu.Lock()
	if el, ok := c.entries[k]; ok {
		c.order.MoveToFront(el)
		res := el.Value.(*entry).res
		c.mu.Unlock()
		return res, nil
	}
	c.mu.Unlock()

	res, err := Diff(ctx, before, after, opts)
	if err != nil {
		return res, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[k]; !ok {
		c.entries[k] = c.order.PushFront(&entry{key: k, res: res})
		for c.order.Len() > c.size {
			last := c.order.Back()
			c.order.Remove(last)
			delete(c.entries, last.Value.(*entry).key)
		}
	}
	return res, nil
}
