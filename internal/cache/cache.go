package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/promptledger/internal/provider"
)

// CachedEntry stores cached LLM outputs with TTL and cost metrics.
type CachedEntry struct {
	Response  *provider.Response
	ExpiresAt time.Time
	CreatedAt time.Time
}

// Stats provides cache performance and financial savings metrics.
type Stats struct {
	TotalLookups     int64   `json:"total_lookups"`
	Hits             int64   `json:"hits"`
	Misses           int64   `json:"misses"`
	HitRatio         float64 `json:"hit_ratio"`
	TotalLatencySavedMs int64 `json:"total_latency_saved_ms"`
	TotalDollarsSaved float64 `json:"total_dollars_saved"`
	CachedItemsCount int     `json:"cached_items_count"`
}

// Cache manages in-memory key-value caching with TTL and financial analytics.
type Cache struct {
	mu    sync.RWMutex
	items map[string]*CachedEntry
	stats Stats
}

// NewCache creates an initialized semantic cache.
func NewCache() *Cache {
	c := &Cache{
		items: make(map[string]*CachedEntry),
	}
	// Background eviction ticker
	go c.evictionLoop()
	return c
}

// GenerateKey computes a canonical deterministic hash for a prompt run.
func GenerateKey(promptID, version string, variables map[string]interface{}) string {
	// Canonicalize variables map order
	var keys []string
	for k := range variables {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString(promptID)
	sb.WriteString(":")
	sb.WriteString(version)
	sb.WriteString(":")

	for _, k := range keys {
		valJSON, _ := json.Marshal(variables[k])
		sb.WriteString(k)
		sb.WriteString("=")
		sb.Write(valJSON)
		sb.WriteString(";")
	}

	hash := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(hash[:])
}

// Get checks the cache for a non-expired entry.
func (c *Cache) Get(key string) (*provider.Response, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stats.TotalLookups++

	entry, exists := c.items[key]
	if !exists {
		c.stats.Misses++
		c.updateRatio()
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		delete(c.items, key)
		c.stats.Misses++
		c.updateRatio()
		return nil, false
	}

	c.stats.Hits++
	c.stats.TotalLatencySavedMs += entry.Response.LatencyMs
	c.stats.TotalDollarsSaved += entry.Response.CostUSD
	c.updateRatio()

	// Return a copy with 1ms cached latency
	cachedResp := *entry.Response
	cachedResp.LatencyMs = 1
	return &cachedResp, true
}

// Set stores a response in the cache with the specified TTL.
func (c *Cache) Set(key string, res *provider.Response, ttl time.Duration) {
	if ttl <= 0 {
		ttl = 1 * time.Hour
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &CachedEntry{
		Response:  res,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}
}

// GetStats returns current lookup, hit ratio, and financial savings data.
func (c *Cache) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	s := c.stats
	s.CachedItemsCount = len(c.items)
	return s
}

// Clear flushes all cached entries.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*CachedEntry)
}

func (c *Cache) updateRatio() {
	if c.stats.TotalLookups > 0 {
		c.stats.HitRatio = float64(c.stats.Hits) / float64(c.stats.TotalLookups)
	}
}

func (c *Cache) evictionLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.items {
			if now.After(v.ExpiresAt) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}
