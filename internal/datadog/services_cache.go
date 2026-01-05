package datadog

import (
	"strings"
	"sync"
	"time"
)

// ServicesCache provides session-level caching for discovered services.
type ServicesCache struct {
	mu        sync.RWMutex
	services  []ServiceInfo
	fetchedAt time.Time
	ttl       time.Duration
}

// Global cache instance
var servicesCache = NewServicesCache(10 * time.Minute)

// NewServicesCache creates a new cache with the specified TTL.
func NewServicesCache(ttl time.Duration) *ServicesCache {
	return &ServicesCache{
		ttl: ttl,
	}
}

// Get returns cached services if available and not expired.
func (c *ServicesCache) Get() ([]ServiceInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.services) == 0 {
		return nil, false
	}

	if time.Since(c.fetchedAt) > c.ttl {
		return nil, false
	}

	// Return a copy to prevent mutation
	result := make([]ServiceInfo, len(c.services))
	copy(result, c.services)
	return result, true
}

// Set stores services in the cache.
func (c *ServicesCache) Set(services []ServiceInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Store a copy to prevent mutation
	c.services = make([]ServiceInfo, len(services))
	copy(c.services, services)
	c.fetchedAt = time.Now()
}

// FilterByEnvPrefix returns services that have environments matching the prefix.
func (c *ServicesCache) FilterByEnvPrefix(prefix string) []ServiceInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if prefix == "" {
		// Return all services
		result := make([]ServiceInfo, len(c.services))
		copy(result, c.services)
		return result
	}

	prefix = strings.ToLower(prefix)
	var filtered []ServiceInfo

	for _, svc := range c.services {
		var matchingEnvs []string
		for _, env := range svc.Environments {
			if strings.HasPrefix(strings.ToLower(env), prefix) {
				matchingEnvs = append(matchingEnvs, env)
			}
		}
		if len(matchingEnvs) > 0 {
			filtered = append(filtered, ServiceInfo{
				Name:         svc.Name,
				Environments: matchingEnvs,
				LastSeen:     svc.LastSeen,
			})
		}
	}

	return filtered
}

// Clear removes all cached data.
func (c *ServicesCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services = nil
	c.fetchedAt = time.Time{}
}

// IsExpired returns true if the cache has expired.
func (c *ServicesCache) IsExpired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.services) == 0 || time.Since(c.fetchedAt) > c.ttl
}

// FetchedAt returns when the cache was last populated.
func (c *ServicesCache) FetchedAt() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.fetchedAt
}

// Package-level functions for accessing the global cache

// GetCachedServices returns cached services, optionally filtered by env prefix.
func GetCachedServices(envPrefix string) ([]ServiceInfo, bool) {
	services, ok := servicesCache.Get()
	if !ok {
		return nil, false
	}
	if envPrefix != "" {
		return servicesCache.FilterByEnvPrefix(envPrefix), true
	}
	return services, true
}

// CacheServices stores services in the global cache.
func CacheServices(services []ServiceInfo) {
	servicesCache.Set(services)
}

// ClearServicesCache clears the global services cache.
func ClearServicesCache() {
	servicesCache.Clear()
}

// IsServicesCacheExpired returns true if the global cache is expired.
func IsServicesCacheExpired() bool {
	return servicesCache.IsExpired()
}

// GetServicesCacheFetchedAt returns when the global cache was last populated.
func GetServicesCacheFetchedAt() time.Time {
	return servicesCache.FetchedAt()
}
