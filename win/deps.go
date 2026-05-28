package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ytDlpBinary  = "yt-dlp.exe"
	ffmpegBinary = "ffmpeg.exe"
	binDir       = "bin"
)

type ReleaseInfo struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func ensureBinDir() error {
	if _, err := os.Stat(binDir); os.IsNotExist(err) {
		return os.MkdirAll(binDir, 0755)
	}
	return nil
}

func getLocalYtDlpVersion() (string, error) {
	path := filepath.Join(binDir, ytDlpBinary)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil // Not installed
	}

	cmd := exec.Command(path, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func getRemoteYtDlpVersion() (string, string, error) {
	resp, err := http.Get("https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest")
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", err
	}

	assetName := ytDlpBinary
	if runtime.GOOS == "windows" {
		assetName = "yt-dlp.exe"
	} else if runtime.GOOS == "darwin" {
		assetName = "yt-dlp_macos" // or similar, usually just yt-dlp works
	} else {
		assetName = "yt-dlp_linux"
	}

	downloadURL := ""
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	// Fallback if specific asset not found (sometimes they change naming)
	if downloadURL == "" {
		for _, asset := range release.Assets {
			if asset.Name == "yt-dlp" {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}
	}

	return release.TagName, downloadURL, nil
}

func checkFFmpeg() bool {
	path := filepath.Join(binDir, ffmpegBinary)
	if _, err := os.Stat(path); err == nil {
		return true
	}
	// Check path
	_, err := exec.LookPath(ffmpegBinary)
	return err == nil
}

// downloadFile downloads a file and reports progress to a callback
func downloadFile(url string, destPath string, onProgress func(float64)) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	size := resp.ContentLength

	// Create a wrapper to track progress
	counter := &WriteCounter{
		Total:      float64(size),
		OnProgress: onProgress,
	}

	if _, err = io.Copy(out, io.TeeReader(resp.Body, counter)); err != nil {
		return err
	}

	return nil
}

type WriteCounter struct {
	Total      float64
	Current    float64
	OnProgress func(float64)
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Current += float64(n)
	if wc.Total > 0 && wc.OnProgress != nil {
		wc.OnProgress(wc.Current / wc.Total)
	}
	return n, nil
}
