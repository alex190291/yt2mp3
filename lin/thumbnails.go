package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

type ThumbnailCache struct {
	mu       sync.Mutex
	dir      string
	pending  map[string][]*canvas.Image
	failed   map[string]time.Time
	cooldown time.Duration
}

func NewThumbnailCache() (*ThumbnailCache, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(base, "ytdl2", "thumbnails")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &ThumbnailCache{
		dir:      dir,
		pending:  make(map[string][]*canvas.Image),
		failed:   make(map[string]time.Time),
		cooldown: 2 * time.Minute,
	}, nil
}

func (c *ThumbnailCache) Load(videoID string, img *canvas.Image) {
	if videoID == "" || img == nil {
		return
	}
	path := c.path(videoID)
	if _, err := os.Stat(path); err == nil {
		img.File = path
		img.Resource = nil
		img.Refresh()
		return
	}

	c.mu.Lock()
	if lastFail, ok := c.failed[videoID]; ok && time.Since(lastFail) < c.cooldown {
		c.mu.Unlock()
		return
	}
	if waiters, ok := c.pending[videoID]; ok {
		c.pending[videoID] = append(waiters, img)
		c.mu.Unlock()
		return
	}
	c.pending[videoID] = []*canvas.Image{img}
	c.mu.Unlock()

	go c.fetch(videoID, path)
}

func (c *ThumbnailCache) path(videoID string) string {
	return filepath.Join(c.dir, fmt.Sprintf("%s.jpg", videoID))
}

func (c *ThumbnailCache) fetch(videoID, path string) {
	url := fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", videoID)
	resp, err := http.Get(url)
	if err != nil {
		c.markFailed(videoID)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.markFailed(videoID)
		return
	}

	tmp := path + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		c.markFailed(videoID)
		return
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		c.markFailed(videoID)
		return
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		c.markFailed(videoID)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		c.markFailed(videoID)
		return
	}

	c.mu.Lock()
	images := c.pending[videoID]
	delete(c.pending, videoID)
	delete(c.failed, videoID)
	c.mu.Unlock()

	fyne.Do(func() {
		for _, img := range images {
			if img == nil {
				continue
			}
			img.File = path
			img.Resource = nil
			img.Refresh()
		}
	})
}

func (c *ThumbnailCache) markFailed(videoID string) {
	c.mu.Lock()
	c.failed[videoID] = time.Now()
	delete(c.pending, videoID)
	c.mu.Unlock()
}

func (c *ThumbnailCache) Remove(videoID string) error {
	if videoID == "" {
		return errors.New("video ID required")
	}
	path := c.path(videoID)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
