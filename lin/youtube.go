package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getYtDlpPath() string {
	localPath := filepath.Join("bin", "yt-dlp")
	if _, err := os.Stat(localPath); err == nil {
		localPath, _ = filepath.Abs(localPath)
		return localPath
	}
	return "yt-dlp"
}

// SearchYouTube searches for videos using yt-dlp.
func SearchYouTube(query string, limit int) ([]Video, error) {
	return searchYouTube(query, limit, "ytsearch")
}

// SearchYouTubePlaylists searches for playlists using yt-dlp.
func SearchYouTubePlaylists(query string, limit int) ([]Video, error) {
	results, err := searchYouTube(query, limit, "ytsearchall")
	if err == nil {
		return filterPlaylists(results), nil
	}
	results, urlErr := searchYouTubePlaylistsURL(query, limit)
	if urlErr != nil {
		return nil, fmt.Errorf("playlist search failed (%v); fallback failed (%v)", err, urlErr)
	}
	return filterPlaylists(results), nil
}

func searchYouTube(query string, limit int, prefix string) ([]Video, error) {
	// yt-dlp "<prefix><limit>:<query>" --flat-playlist --dump-json
	searchQuery := fmt.Sprintf("%s%d:%s", prefix, limit, query)
	cmd := exec.Command(getYtDlpPath(), searchQuery, "--flat-playlist", "--dump-json", "--no-warnings")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}

	return parseYtDlpJSON(output)
}

// FetchPlaylist fetches videos from a playlist URL
func FetchPlaylist(url string, limit int) ([]Video, error) {
	args := []string{url, "--flat-playlist", "--dump-json", "--no-warnings"}
	if limit > 0 {
		args = append(args, fmt.Sprintf("--playlist-end=%d", limit))
	}

	cmd := exec.Command(getYtDlpPath(), args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseYtDlpJSON(output)
}

func parseYtDlpJSON(data []byte) ([]Video, error) {
	var videos []Video
	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		line := scanner.Bytes()
		var v Video
		if err := json.Unmarshal(line, &v); err == nil {
			normalizeSearchResult(&v)
			v.IsPlaylist = isPlaylistResult(v)
			videos = append(videos, v)
		}
	}

	return videos, scanner.Err()
}

func searchYouTubePlaylistsURL(query string, limit int) ([]Video, error) {
	escaped := url.QueryEscape(query)
	searchURL := fmt.Sprintf("https://www.youtube.com/results?search_query=%s&sp=EgIQAw%%253D%%253D", escaped)
	args := []string{searchURL, "--flat-playlist", "--dump-json", "--no-warnings"}
	if limit > 0 {
		args = append(args, fmt.Sprintf("--playlist-end=%d", limit))
	}
	cmd := exec.Command(getYtDlpPath(), args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return parseYtDlpJSON(output)
}

func filterPlaylists(results []Video) []Video {
	playlists := make([]Video, 0, len(results))
	for _, v := range results {
		if v.IsPlaylist {
			playlists = append(playlists, v)
		}
	}
	return playlists
}

func normalizeSearchResult(v *Video) {
	if v.URL == "" && v.WebpageURL != "" {
		v.URL = v.WebpageURL
	}
	if v.URL != "" && !strings.HasPrefix(v.URL, "http") {
		if isPlaylistCandidate(*v) {
			v.URL = "https://www.youtube.com/playlist?list=" + v.URL
		} else {
			v.URL = "https://www.youtube.com/watch?v=" + v.URL
		}
	}
	// Construct URL if missing (flat-playlist often just gives ID)
	if v.URL == "" && v.ID != "" {
		if isPlaylistCandidate(*v) {
			v.URL = "https://www.youtube.com/playlist?list=" + v.ID
		} else {
			v.URL = "https://www.youtube.com/watch?v=" + v.ID
		}
	}
}

func isPlaylistResult(v Video) bool {
	if isPlaylistCandidate(v) {
		return true
	}
	if hasPlaylistURL(v.URL) {
		return true
	}
	return false
}

func isPlaylistCandidate(v Video) bool {
	if v.Type == "playlist" || v.PlaylistCount > 0 || v.EntriesCount > 0 {
		return true
	}
	if hasPlaylistURL(v.WebpageURL) {
		return true
	}
	return false
}

func hasPlaylistURL(raw string) bool {
	if raw == "" {
		return false
	}
	if strings.Contains(raw, "playlist?list=") {
		return true
	}
	if strings.Contains(raw, "list=") && !strings.Contains(raw, "watch?v=") {
		return true
	}
	return false
}

func Download(ctx context.Context, req DownloadRequest) *exec.Cmd {
	args := []string{req.URL, "--no-warnings", "--progress", "--newline"}
	switch req.Format {
	case FormatAudioMP3:
		args = append(args, "-x", "--audio-format", "mp3")
	case FormatVideoMP4:
		args = append(args, "-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]/best")
	case FormatBest:
		// Use yt-dlp default selection.
	}
	if req.OutputDir != "" {
		args = append(args, "-P", req.OutputDir)
	}
	if req.FilenameTemplate != "" {
		args = append(args, "-o", req.FilenameTemplate)
	}
	// Also set ffmpeg location if local
	localFFmpeg := filepath.Join("bin", "ffmpeg")
	if _, err := os.Stat(localFFmpeg); err == nil {
		localFFmpeg, _ = filepath.Abs(localFFmpeg)
		args = append(args, "--ffmpeg-location", localFFmpeg)
	}

	return exec.CommandContext(ctx, getYtDlpPath(), args...)
}
