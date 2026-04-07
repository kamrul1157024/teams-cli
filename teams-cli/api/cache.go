package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CacheEntry wraps cached data with TTL metadata
type CacheEntry struct {
	Key       string          `json:"key"`
	ExpiresAt time.Time       `json:"expires_at"`
	Data      json.RawMessage `json:"data"`
}

// CacheConfig controls caching behavior
type CacheConfig struct {
	Enabled bool // false = --no-cache
	Refresh bool // true = --refresh (ignore cache, write new)
}

// DefaultTTLs for different data types
const (
	TTLConversations = 5 * time.Minute  // Chat list — changes moderately
	TTLUser          = 1 * time.Hour    // User info — very stable
	TTLMe            = 1 * time.Hour    // Own profile — very stable
	TTLTeams         = 30 * time.Minute // Team structure — stable
	TTLChannels      = 30 * time.Minute // Channels — stable
	TTLResolveChat   = 10 * time.Minute // Email-to-chat-ID — stable
)

func cacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "teams-cli", "cache")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

func cacheKey(prefix string, args ...string) string {
	h := sha256.New()
	h.Write([]byte(prefix))
	for _, a := range args {
		h.Write([]byte("|"))
		h.Write([]byte(a))
	}
	return fmt.Sprintf("%s_%x", prefix, h.Sum(nil)[:8])
}

func cachePath(key string) (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, key+".json"), nil
}

// cacheGet reads cached data if it exists and hasn't expired.
// Returns nil if cache miss, expired, or disabled.
func (c *Client) cacheGet(key string, result interface{}) bool {
	if !c.cache.Enabled || c.cache.Refresh {
		return false
	}

	path, err := cachePath(key)
	if err != nil {
		return false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return false
	}

	if time.Now().After(entry.ExpiresAt) {
		// Expired — remove stale file
		os.Remove(path)
		return false
	}

	if err := json.Unmarshal(entry.Data, result); err != nil {
		return false
	}

	return true
}

// cacheSet writes data to cache with the given TTL
func (c *Client) cacheSet(key string, data interface{}, ttl time.Duration) {
	if !c.cache.Enabled {
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	entry := CacheEntry{
		Key:       key,
		ExpiresAt: time.Now().Add(ttl),
		Data:      jsonData,
	}

	entryData, err := json.Marshal(entry)
	if err != nil {
		return
	}

	path, err := cachePath(key)
	if err != nil {
		return
	}

	os.WriteFile(path, entryData, 0600)
}

// CacheInvalidate removes specific cache entries by prefix
func (c *Client) CacheInvalidate(prefixes ...string) {
	dir, err := cacheDir()
	if err != nil {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		name := e.Name()
		for _, prefix := range prefixes {
			if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
				os.Remove(filepath.Join(dir, name))
				break
			}
		}
	}
}

// CacheClear removes all cache files
func CacheClear() error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		os.Remove(filepath.Join(dir, e.Name()))
	}
	return nil
}
