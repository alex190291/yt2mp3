package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
)

type VideoInfo struct {
	Title          string       `json:"title"`
	Duration       float64      `json:"duration"`
	Filesize       int64        `json:"filesize"`
	FilesizeApprox int64        `json:"filesize_approx"`
	Formats        []FormatInfo `json:"formats"`
}

type FormatInfo struct {
	FormatID       string  `json:"format_id"`
	Ext            string  `json:"ext"`
	Resolution     string  `json:"resolution"`
	Height         int     `json:"height"`
	Filesize       int64   `json:"filesize"`
	FilesizeApprox int64   `json:"filesize_approx"`
	ACodec         string  `json:"acodec"`
	VCodec         string  `json:"vcodec"`
	FormatNote     string  `json:"format_note"`
	TBR            float64 `json:"tbr"`
	ABR            float64 `json:"abr"`
}

type FormatPreview struct {
	ID         string
	Ext        string
	Resolution string
	Size       string
	Note       string
}

func FetchVideoInfo(videoURL string) (*VideoInfo, error) {
	cmd := configureCommand(exec.Command(getYtDlpPath(), videoURL, "--dump-json", "--no-warnings"))
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var info VideoInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func BuildFormatPreviews(info *VideoInfo, max int) []FormatPreview {
	if info == nil || len(info.Formats) == 0 {
		return nil
	}
	formats := make([]FormatInfo, 0, len(info.Formats))
	for _, f := range info.Formats {
		if f.Ext == "" || f.FormatID == "" {
			continue
		}
		formats = append(formats, f)
	}
	sort.Slice(formats, func(i, j int) bool {
		if formats[i].Height != formats[j].Height {
			return formats[i].Height > formats[j].Height
		}
		return formats[i].TBR > formats[j].TBR
	})

	picks := make([]FormatPreview, 0, max)
	for _, f := range formats {
		if len(picks) >= max {
			break
		}
		if f.Ext != "mp4" && f.Ext != "webm" && f.Ext != "m4a" {
			continue
		}
		size := formatBytes(bestSize(f.Filesize, f.FilesizeApprox))
		note := f.FormatNote
		if note == "" {
			note = f.Resolution
		}
		picks = append(picks, FormatPreview{
			ID:         f.FormatID,
			Ext:        f.Ext,
			Resolution: f.Resolution,
			Size:       size,
			Note:       note,
		})
	}
	return picks
}

func EstimateSize(info *VideoInfo, format DownloadFormat) string {
	if info == nil {
		return "—"
	}
	switch format {
	case FormatAudioMP3:
		if audio := pickBestAudio(info); audio != nil {
			return formatBytes(bestSize(audio.Filesize, audio.FilesizeApprox))
		}
	case FormatVideoMP4:
		if video := pickBestMP4(info); video != nil {
			return formatBytes(bestSize(video.Filesize, video.FilesizeApprox))
		}
	case FormatBest:
		if info.Filesize > 0 || info.FilesizeApprox > 0 {
			return formatBytes(bestSize(info.Filesize, info.FilesizeApprox))
		}
		if best := pickBestOverall(info); best != nil {
			return formatBytes(bestSize(best.Filesize, best.FilesizeApprox))
		}
	}
	return "—"
}

func pickBestAudio(info *VideoInfo) *FormatInfo {
	var best *FormatInfo
	for i := range info.Formats {
		f := &info.Formats[i]
		if f.VCodec != "none" || f.ACodec == "none" {
			continue
		}
		if best == nil {
			best = f
			continue
		}
		if f.ABR > best.ABR || f.TBR > best.TBR || bestSize(f.Filesize, f.FilesizeApprox) > bestSize(best.Filesize, best.FilesizeApprox) {
			best = f
		}
	}
	return best
}

func pickBestMP4(info *VideoInfo) *FormatInfo {
	var best *FormatInfo
	for i := range info.Formats {
		f := &info.Formats[i]
		if f.Ext != "mp4" || f.VCodec == "none" {
			continue
		}
		if best == nil {
			best = f
			continue
		}
		if f.Height > best.Height || f.TBR > best.TBR {
			best = f
		}
	}
	return best
}

func pickBestOverall(info *VideoInfo) *FormatInfo {
	var best *FormatInfo
	for i := range info.Formats {
		f := &info.Formats[i]
		if best == nil {
			best = f
			continue
		}
		if f.Height > best.Height || f.TBR > best.TBR {
			best = f
		}
	}
	return best
}

func bestSize(primary, fallback int64) int64 {
	if primary > 0 {
		return primary
	}
	return fallback
}

func formatBytes(size int64) string {
	if size <= 0 {
		return "—"
	}
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}
