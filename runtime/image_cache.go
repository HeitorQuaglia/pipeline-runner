package runtime

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

type ImageCache struct {
	cachedImages map[string]ImageInfo
	logger       *logrus.Logger
	mutex        sync.RWMutex
	stats        CacheStats
}

type ImageInfo struct {
	ImageID  string
	CachedAt time.Time
	LastUsed time.Time
	UseCount int
}

type CacheStats struct {
	Hits   uint64
	Misses uint64
	Pulls  uint64
}

func NewImageCache(logger *logrus.Logger) *ImageCache {
	return &ImageCache{
		cachedImages: make(map[string]ImageInfo),
		logger:       logger,
		stats:        CacheStats{},
	}
}

func (ic *ImageCache) IsImageCached(ctx context.Context, client *client.Client, imageName string) (bool, error) {
	ic.mutex.RLock()
	info, exists := ic.cachedImages[imageName]
	ic.mutex.RUnlock()

	if !exists {
		atomic.AddUint64(&ic.stats.Misses, 1)
		return false, nil
	}

	_, _, err := client.ImageInspectWithRaw(ctx, info.ImageID)
	if err != nil {
		ic.mutex.Lock()
		delete(ic.cachedImages, imageName)
		ic.mutex.Unlock()

		ic.logger.Debugf("Image %s removed from cache (not found in Docker)", imageName)
		atomic.AddUint64(&ic.stats.Misses, 1)
		return false, nil
	}

	ic.mutex.Lock()
	info.LastUsed = time.Now()
	info.UseCount++
	ic.cachedImages[imageName] = info
	ic.mutex.Unlock()

	atomic.AddUint64(&ic.stats.Hits, 1)
	ic.logger.Debugf("Image cache hit: %s (used %d times)", imageName, info.UseCount)
	return true, nil
}

func (ic *ImageCache) AddToCache(ctx context.Context, client *client.Client, imageName string) error {
	imageInfo, _, err := client.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		return fmt.Errorf("failed to inspect image %s: %w", imageName, err)
	}

	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	now := time.Now()
	ic.cachedImages[imageName] = ImageInfo{
		ImageID:  imageInfo.ID,
		CachedAt: now,
		LastUsed: now,
		UseCount: 1,
	}

	ic.logger.Debugf("Added image %s to cache (ID: %s)", imageName, imageInfo.ID[:12])
	return nil
}

func (ic *ImageCache) GetStats() CacheStats {
	return CacheStats{
		Hits:   atomic.LoadUint64(&ic.stats.Hits),
		Misses: atomic.LoadUint64(&ic.stats.Misses),
		Pulls:  atomic.LoadUint64(&ic.stats.Pulls),
	}
}

func (ic *ImageCache) PrintStats() {
	stats := ic.GetStats()
	total := stats.Hits + stats.Misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(stats.Hits) / float64(total) * 100
	}

	ic.logger.Infof("Image Cache Stats - Hits: %d, Misses: %d, Pulls: %d, Hit Rate: %.1f%%",
		stats.Hits, stats.Misses, stats.Pulls, hitRate)
}

func (ic *ImageCache) ListCachedImages() []string {
	ic.mutex.RLock()
	defer ic.mutex.RUnlock()

	images := make([]string, 0, len(ic.cachedImages))
	for imageName := range ic.cachedImages {
		images = append(images, imageName)
	}
	return images
}

func (ic *ImageCache) IsImageCachedSimple(imageName string) bool {
	ic.mutex.RLock()
	_, exists := ic.cachedImages[imageName]
	ic.mutex.RUnlock()
	return exists
}

func (ic *ImageCache) UpdateLastUsed(imageName string) {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	if info, exists := ic.cachedImages[imageName]; exists {
		info.LastUsed = time.Now()
		info.UseCount++
		ic.cachedImages[imageName] = info
	}
}

func (ic *ImageCache) AddImage(ctx context.Context, client *client.Client, imageName string) error {
	return ic.AddToCache(ctx, client, imageName)
}