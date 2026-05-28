package main

type Video struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Uploader      string `json:"uploader"`
	Duration      string `json:"duration_string"` // yt-dlp returns formatted duration often
	URL           string `json:"url"`             // usually computed from ID
	WebpageURL    string `json:"webpage_url"`
	Type          string `json:"_type"`
	Channel       string `json:"channel"`
	PlaylistCount int    `json:"playlist_count"`
	EntriesCount  int    `json:"n_entries"`
	IsPlaylist    bool   `json:"-"`
}

func (v Video) FilterValue() string { return v.Title }
func (v Video) TitleString() string { return v.Title }
func (v Video) DescriptionString() string { return v.Uploader + " • " + v.Duration }

type Playlist struct {
	ID    string
	Title string
	Videos []Video
}

type DownloadOption struct {
	IsPlaylist bool
	Limit      int // 0 for unlimited
	AudioOnly  bool
}
