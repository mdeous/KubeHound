package cache

import (
	"context"
	"sync"

	"github.com/DataDog/KubeHound/pkg/kubehound/storage/cache/cachekey"
	"github.com/DataDog/KubeHound/pkg/telemetry/log"
	"github.com/DataDog/KubeHound/pkg/telemetry/metric"
	"github.com/DataDog/KubeHound/pkg/telemetry/statsd"
	"github.com/DataDog/KubeHound/pkg/telemetry/tag"
)

type MemCacheAsyncWriter struct {
	data map[string]any
	mu   *sync.RWMutex
	opts *writerOptions
}

// To support the object writing, we will need at least redis 4 which support the HSET function
func (m *MemCacheAsyncWriter) Queue(ctx context.Context, key cachekey.CacheKey, value any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tagCacheKey := tag.GetBaseTagsWith(tag.CacheKey(key.Shard()))
	_ = statsd.Incr(ctx, metric.CacheWrite, tagCacheKey, 1)
	keyId := computeKey(key)
	entry, ok := m.data[keyId]
	if ok {
		if m.opts.Test {
			// if test & set behaviour is specified, return an error containing the existing value in the cache
			return NewOverwriteError(&CacheResult{Value: entry})
		}

		if !m.opts.ExpectOverwrite {
			// if overwrite is expected (e.g fast tracking of existence regardless of value), suppress metrics and logs
			_ = statsd.Incr(ctx, metric.CacheDuplicate, tagCacheKey, 1)
			log.Trace(ctx).Warnf("overwriting cache entry key=%s old=%#v new=%#v", keyId, entry, value)
		}
	}

	m.data[keyId] = value

	return nil
}

func (m *MemCacheAsyncWriter) Flush(_ context.Context) error {
	return nil
}

func (m *MemCacheAsyncWriter) Close(_ context.Context) error {
	// Underlying data map is owned by the proviuder object
	return nil
}
