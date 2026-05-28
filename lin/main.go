package main

import (
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func main() {
	myApp := app.New()
	myApp.Settings().SetTheme(&AppTheme{})

	w := myApp.NewWindow("YTDL2")
	w.Resize(fyne.NewSize(1100, 720))

	runOnMain := func(fn func()) {
		fyne.Do(fn)
	}

	downloadFormat := FormatAudioMP3
	outputDir := ""
	filenameTemplate := "%(title)s.%(ext)s"

	manager := NewDownloadManager(2, nil)

	statusLabel := widget.NewLabel("Ready")
	statusLabel.Alignment = fyne.TextAlignLeading
	statusCounts := widget.NewLabel("")
	statusCounts.Alignment = fyne.TextAlignTrailing
	statusCounts.SetText("0 active • 0 queued • 0 history")

	var queueItems []*DownloadJob
	var activeItems []*DownloadJob
	var historyItems []*DownloadJob

	queueEmpty := widget.NewLabel("Queue is empty. Add a video to start.")
	queueEmpty.Alignment = fyne.TextAlignCenter
	historyEmpty := widget.NewLabel("No history yet.")
	historyEmpty.Alignment = fyne.TextAlignCenter
	resultsEmpty := widget.NewLabel("Search results appear here.")
	resultsEmpty.Alignment = fyne.TextAlignCenter

	queueSummaryEmpty := widget.NewLabel("No active downloads.")
	queueSummaryEmpty.Alignment = fyne.TextAlignCenter

	thumbCache, thumbErr := NewThumbnailCache()
	if thumbErr != nil {
		statusLabel.SetText("Thumbnail cache disabled: " + thumbErr.Error())
	}
	stateStore, stateErr := NewStateStore("ytdl2")
	if stateErr != nil {
		statusLabel.SetText("State store disabled: " + stateErr.Error())
	}

	buildRequest := func(url, title string) DownloadRequest {
		return DownloadRequest{
			URL:              url,
			Title:            title,
			Format:           downloadFormat,
			OutputDir:        outputDir,
			FilenameTemplate: filenameTemplate,
		}
	}

	isSupportedURL := func(raw string) bool {
		parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
		if err != nil {
			return false
		}
		return parsed.Scheme == "http" || parsed.Scheme == "https"
	}

	// --- Search Tab ---
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search YouTube videos or playlists (press Enter)")

	limitSelect := widget.NewSelect([]string{"10", "15", "25", "50"}, nil)
	limitSelect.SetSelected("15")
	searchTypeSelect := widget.NewSelect([]string{"Videos", "Playlists"}, nil)
	searchTypeSelect.SetSelected("Videos")

	formatHintSearch := widget.NewLabel(fmt.Sprintf("Queue format: %s", downloadFormat))
	formatHintURL := widget.NewLabel(fmt.Sprintf("Queue format: %s", downloadFormat))

	var selectedInfo *VideoInfo
	var selectedID string
	var formatPreviews []FormatPreview
	playlistCount := func(vid Video) int {
		if vid.PlaylistCount > 0 {
			return vid.PlaylistCount
		}
		if vid.EntriesCount > 0 {
			return vid.EntriesCount
		}
		return 0
	}
	formatResultMeta := func(vid Video) string {
		if vid.IsPlaylist {
			return fmt.Sprintf("Add playlist as %s", downloadFormat)
		}
		return fmt.Sprintf("Add as %s", downloadFormat)
	}
	formatResultSubtitle := func(vid Video) string {
		if vid.IsPlaylist {
			owner := vid.Uploader
			if owner == "" {
				owner = vid.Channel
			}
			count := playlistCount(vid)
			if owner != "" && count > 0 {
				return fmt.Sprintf("%s • %d items", owner, count)
			}
			if owner != "" {
				return owner + " • Playlist"
			}
			if count > 0 {
				return fmt.Sprintf("Playlist • %d items", count)
			}
			return "Playlist"
		}
		if vid.Uploader != "" && vid.Duration != "" {
			return fmt.Sprintf("%s • %s", vid.Uploader, vid.Duration)
		}
		if vid.Uploader != "" {
			return vid.Uploader
		}
		if vid.Duration != "" {
			return vid.Duration
		}
		return "Video"
	}

	detailThumb := canvas.NewImageFromResource(theme.MediaPhotoIcon())
	detailThumb.FillMode = canvas.ImageFillCover
	detailThumb.ScaleMode = canvas.ImageScaleSmooth
	detailThumb.CornerRadius = 6
	detailThumbBg := canvas.NewRectangle(colSurfaceAlt)
	detailThumbBg.CornerRadius = 6
	detailThumbWrap := container.NewGridWrap(fyne.NewSize(240, 135), container.NewMax(detailThumbBg, detailThumb))

	detailTitle := widget.NewLabel("Select a result")
	detailTitle.TextStyle = fyne.TextStyle{Bold: true}
	detailTitle.Wrapping = fyne.TextWrapWord
	detailMeta := widget.NewLabel("Choose a result to preview formats and size.")
	detailMeta.Wrapping = fyne.TextWrapWord
	detailFormat := widget.NewLabel(fmt.Sprintf("Format: %s", downloadFormat))
	detailSize := widget.NewLabel("Estimated size: —")
	detailStatus := widget.NewLabel("")

	detailQueueBtn := widget.NewButtonWithIcon("Queue Selection", theme.ContentAddIcon(), nil)
	detailQueueBtn.Importance = widget.HighImportance
	detailQueueBtn.Disable()
	detailRefreshBtn := widget.NewButtonWithIcon("Refresh Preview", theme.ViewRefreshIcon(), nil)
	detailRefreshBtn.Disable()

	formatList := widget.NewList(
		func() int { return len(formatPreviews) },
		func() fyne.CanvasObject {
			label := widget.NewLabel("mp4 • 1080p • 123 MiB")
			return label
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(formatPreviews) {
				return
			}
			preview := formatPreviews[id]
			label := item.(*widget.Label)
			parts := preview.Ext
			if preview.Resolution != "" {
				parts += " • " + preview.Resolution
			}
			if preview.Size != "" {
				parts += " • " + preview.Size
			}
			if preview.Note != "" && preview.Note != preview.Resolution {
				parts += " • " + preview.Note
			}
			label.SetText(parts)
		},
	)

	updatePreview := func(vid Video) {
		selectedID = vid.ID
		selectedInfo = nil
		detailTitle.SetText(vid.Title)
		detailMeta.SetText(formatResultSubtitle(vid))
		detailFormat.SetText(fmt.Sprintf("Format: %s", downloadFormat))
		detailSize.SetText("Estimated size: —")
		detailQueueBtn.Enable()

		detailQueueBtn.OnTapped = func() {
			manager.Add(buildRequest(vid.URL, vid.Title))
			statusLabel.SetText("Queued: " + vid.Title)
		}
		detailRefreshBtn.OnTapped = func() {
			detailStatus.SetText("Refreshing preview...")
			go func(requestID string, url string) {
				info, err := FetchVideoInfo(url)
				runOnMain(func() {
					if selectedID != requestID {
						return
					}
					if err != nil {
						selectedInfo = nil
						detailStatus.SetText("Format preview unavailable")
						detailSize.SetText("Estimated size: —")
						formatPreviews = nil
						formatList.Refresh()
						return
					}
					selectedInfo = info
					formatPreviews = BuildFormatPreviews(info, 6)
					detailSize.SetText("Estimated size: " + EstimateSize(info, downloadFormat))
					if len(formatPreviews) == 0 {
						detailStatus.SetText("No preview formats found")
					} else {
						detailStatus.SetText("Preview ready")
					}
					formatList.Refresh()
				})
			}(vid.ID, vid.URL)
		}

		if thumbCache != nil {
			detailThumb.Resource = theme.MediaPhotoIcon()
			detailThumb.File = ""
			detailThumb.Refresh()
			if !vid.IsPlaylist {
				thumbCache.Load(vid.ID, detailThumb)
			}
		}

		if vid.IsPlaylist {
			detailStatus.SetText("Format preview unavailable for playlists")
			detailRefreshBtn.Disable()
			detailRefreshBtn.OnTapped = nil
			formatPreviews = nil
			formatList.Refresh()
			return
		}

		detailStatus.SetText("Loading format preview...")
		detailRefreshBtn.Enable()
		detailRefreshBtn.OnTapped()
	}

	var searchResults []Video
	resultList := widget.NewList(
		func() int { return len(searchResults) },
		func() fyne.CanvasObject {
			thumbBg := canvas.NewRectangle(colSurfaceAlt)
			thumbBg.CornerRadius = 8
			thumbImg := canvas.NewImageFromResource(theme.MediaPhotoIcon())
			thumbImg.FillMode = canvas.ImageFillCover
			thumbImg.ScaleMode = canvas.ImageScaleSmooth
			thumbImg.CornerRadius = 8
			thumb := container.NewMax(thumbBg, thumbImg)
			thumbWrap := container.NewGridWrap(fyne.NewSize(96, 54), thumb)

			title := widget.NewLabel("Title")
			title.TextStyle = fyne.TextStyle{Bold: true}
			title.Wrapping = fyne.TextWrapWord
			subtitle := widget.NewLabel("Uploader • Duration")
			meta := widget.NewLabel("Add as Audio (MP3)")

			labels := container.NewVBox(title, subtitle, meta)
			addBtn := widget.NewButtonWithIcon("Queue", theme.ContentAddIcon(), nil)
			addBtn.Importance = widget.HighImportance

			row := container.NewHBox(thumbWrap, labels, layout.NewSpacer(), addBtn)
			return NewPanel(row)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(searchResults) {
				return
			}
			vid := searchResults[id]
			card := item.(*fyne.Container)
			padded := card.Objects[2].(*fyne.Container)
			row := padded.Objects[0].(*fyne.Container)
			labels := row.Objects[1].(*fyne.Container)

			title := labels.Objects[0].(*widget.Label)
			subtitle := labels.Objects[1].(*widget.Label)
			meta := labels.Objects[2].(*widget.Label)
			addBtn := row.Objects[3].(*widget.Button)

			title.SetText(vid.Title)
			subtitle.SetText(formatResultSubtitle(vid))
			meta.SetText(formatResultMeta(vid))
			addBtn.OnTapped = func() {
				manager.Add(buildRequest(vid.URL, vid.Title))
				statusLabel.SetText("Queued: " + vid.Title)
			}

			thumbWrap := row.Objects[0].(*fyne.Container)
			thumbStack := thumbWrap.Objects[0].(*fyne.Container)
			thumbImg := thumbStack.Objects[1].(*canvas.Image)
			thumbImg.Resource = theme.MediaPhotoIcon()
			thumbImg.File = ""
			thumbImg.Refresh()
			if thumbCache != nil && !vid.IsPlaylist {
				thumbCache.Load(vid.ID, thumbImg)
			}
		},
	)
	resultList.OnSelected = func(id widget.ListItemID) {
		if id >= len(searchResults) {
			return
		}
		updatePreview(searchResults[id])
	}

	searchBtn := widget.NewButtonWithIcon("Search", theme.SearchIcon(), nil)
	runSearch := func() {
		query := strings.TrimSpace(searchEntry.Text)
		if query == "" {
			return
		}
		statusLabel.SetText("Searching...")
		searchBtn.Disable()
		resultsEmpty.Hide()

		limit, _ := strconv.Atoi(limitSelect.Selected)
		searchPlaylists := searchTypeSelect.Selected == "Playlists"
		go func() {
			var res []Video
			var err error
			if searchPlaylists {
				res, err = SearchYouTubePlaylists(query, limit)
			} else {
				res, err = SearchYouTube(query, limit)
			}
			runOnMain(func() {
				searchBtn.Enable()
				if err != nil {
					statusLabel.SetText("Search error: " + err.Error())
					searchResults = nil
					resultsEmpty.SetText("Search failed. Check yt-dlp and your network, then try again.")
					resultsEmpty.Show()
					resultList.Refresh()
					return
				}
				searchResults = res
				if len(searchResults) == 0 {
					resultsEmpty.SetText("No matching results.")
					resultsEmpty.Show()
				} else {
					resultsEmpty.Hide()
				}
				resultList.Refresh()
				if searchPlaylists {
					statusLabel.SetText(fmt.Sprintf("Found %d playlists", len(res)))
				} else {
					statusLabel.SetText(fmt.Sprintf("Found %d results", len(res)))
				}
			})
		}()
	}
	searchBtn.OnTapped = runSearch
	searchBtn.Importance = widget.HighImportance
	searchEntry.OnSubmitted = func(string) {
		runSearch()
	}

	searchInput := container.NewBorder(nil, nil, nil, nil, searchEntry)
	searchControls := container.NewHBox(searchBtn, searchTypeSelect, limitSelect, layout.NewSpacer())
	searchHeader := container.NewVBox(searchInput, searchControls)
	searchMeta := container.NewHBox(formatHintSearch, layout.NewSpacer())
	resultsStack := container.NewStack(resultsEmpty, resultList)
	searchContent := container.NewBorder(container.NewVBox(searchHeader, searchMeta), nil, nil, nil, resultsStack)

	detailCard := NewPanel(container.NewVBox(
		widget.NewLabelWithStyle("Preview", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		detailThumbWrap,
		detailTitle,
		detailMeta,
		detailFormat,
		detailSize,
		detailStatus,
		widget.NewLabelWithStyle("Format Preview", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formatList,
		container.NewHBox(detailQueueBtn, detailRefreshBtn),
	))
	searchSplit := container.NewHSplit(NewPanel(searchContent), detailCard)
	searchSplit.SetOffset(0.62)

	// --- Direct URL Tab ---
	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("Paste YouTube link here")
	urlAddBtn := widget.NewButtonWithIcon("Add to Queue", theme.ContentAddIcon(), func() {
		link := strings.TrimSpace(urlEntry.Text)
		if link == "" {
			return
		}
		if !isSupportedURL(link) {
			dialog.ShowError(fmt.Errorf("enter a valid http or https URL"), w)
			return
		}
		manager.Add(buildRequest(link, link))
		statusLabel.SetText("Queued: " + link)
		urlEntry.SetText("")
	})
	urlAddBtn.Importance = widget.HighImportance

	urlCard := NewPanel(container.NewVBox(
		widget.NewLabelWithStyle("Direct Download", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		urlEntry,
		container.NewHBox(formatHintURL),
		layout.NewSpacer(),
		urlAddBtn,
	))

	// --- Queue Tab ---
	queueList := widget.NewList(
		func() int { return len(queueItems) },
		func() fyne.CanvasObject {
			title := widget.NewLabel("Title")
			title.TextStyle = fyne.TextStyle{Bold: true}
			title.Wrapping = fyne.TextWrapWord
			status := widget.NewLabel("Queued")
			header := container.NewHBox(title, layout.NewSpacer(), status)

			formatLabel := widget.NewLabel("Format: Audio (MP3)")
			message := widget.NewLabel("Waiting...")
			progress := NewFancyProgressBar()
			speed := widget.NewLabel("Speed —")
			eta := widget.NewLabel("ETA —")
			meta := container.NewHBox(speed, layout.NewSpacer(), eta)

			pauseBtn := widget.NewButtonWithIcon("Pause", theme.MediaPauseIcon(), nil)
			cancelBtn := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), nil)
			retryBtn := widget.NewButtonWithIcon("Retry", theme.ViewRefreshIcon(), nil)
			actions := container.NewHBox(pauseBtn, cancelBtn, retryBtn)

			content := container.NewVBox(header, formatLabel, message, progress, meta, actions)
			return NewPanel(content)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(queueItems) {
				return
			}
			job := queueItems[id]
			card := item.(*fyne.Container)
			padded := card.Objects[2].(*fyne.Container)
			content := padded.Objects[0].(*fyne.Container)

			header := content.Objects[0].(*fyne.Container)
			title := header.Objects[0].(*widget.Label)
			status := header.Objects[2].(*widget.Label)

			formatLabel := content.Objects[1].(*widget.Label)
			message := content.Objects[2].(*widget.Label)
			progress := content.Objects[3].(*FancyProgressBar)
			meta := content.Objects[4].(*fyne.Container)
			speed := meta.Objects[0].(*widget.Label)
			eta := meta.Objects[2].(*widget.Label)

			actions := content.Objects[5].(*fyne.Container)
			pauseBtn := actions.Objects[0].(*widget.Button)
			cancelBtn := actions.Objects[1].(*widget.Button)
			retryBtn := actions.Objects[2].(*widget.Button)

			title.SetText(job.Request.Title)
			status.SetText(string(job.Stage))
			formatLabel.SetText(fmt.Sprintf("Format: %s", job.Request.Format))
			messageText := job.Message
			if job.Error != "" {
				messageText = job.Error
			}
			if messageText == "" {
				messageText = string(job.Status)
			}
			message.SetText(messageText)

			progress.SetValue(job.Progress)
			progress.SetIndeterminate(job.Stage == StageConvert || job.Stage == StageQueued || job.Stage == StagePaused)

			if job.Speed != "" {
				speed.SetText("Speed " + job.Speed)
			} else {
				speed.SetText("Speed —")
			}
			if job.ETA != "" {
				eta.SetText("ETA " + job.ETA)
			} else {
				eta.SetText("ETA —")
			}

			cancelBtn.OnTapped = func() { manager.Cancel(job.ID) }
			retryBtn.OnTapped = func() { manager.Add(job.Request) }
			if job.Status == StatusPaused {
				pauseBtn.SetText("Resume")
				pauseBtn.SetIcon(theme.MediaPlayIcon())
				pauseBtn.OnTapped = func() {
					if err := manager.Resume(job.ID); err != nil {
						dialog.ShowError(err, w)
					}
				}
			} else {
				pauseBtn.SetText("Pause")
				pauseBtn.SetIcon(theme.MediaPauseIcon())
				pauseBtn.OnTapped = func() {
					if err := manager.Pause(job.ID); err != nil {
						dialog.ShowError(err, w)
					}
				}
			}

			if job.Status == StatusFailed || job.Status == StatusCanceled {
				retryBtn.Show()
			} else {
				retryBtn.Hide()
			}
			if job.Status == StatusRunning || job.Status == StatusQueued || job.Status == StatusPaused {
				cancelBtn.Show()
			} else {
				cancelBtn.Hide()
			}
			if job.Status == StatusRunning || job.Status == StatusPaused {
				pauseBtn.Show()
			} else {
				pauseBtn.Hide()
			}
		},
	)
	queueStack := container.NewStack(queueEmpty, queueList)

	// --- History Tab ---
	historyList := widget.NewList(
		func() int { return len(historyItems) },
		func() fyne.CanvasObject {
			title := widget.NewLabel("Title")
			title.TextStyle = fyne.TextStyle{Bold: true}
			title.Wrapping = fyne.TextWrapWord
			status := widget.NewLabel("Completed")
			output := widget.NewLabel("Saved to ...")
			formatLabel := widget.NewLabel("Format: Audio (MP3)")
			openBtn := widget.NewButtonWithIcon("Open Folder", theme.FolderOpenIcon(), nil)
			copyBtn := widget.NewButtonWithIcon("Copy Path", theme.ContentCopyIcon(), nil)
			actions := container.NewHBox(openBtn, copyBtn)
			content := container.NewVBox(title, status, formatLabel, output, actions)
			return NewPanel(content)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(historyItems) {
				return
			}
			job := historyItems[id]
			card := item.(*fyne.Container)
			padded := card.Objects[2].(*fyne.Container)
			content := padded.Objects[0].(*fyne.Container)
			title := content.Objects[0].(*widget.Label)
			status := content.Objects[1].(*widget.Label)
			formatLabel := content.Objects[2].(*widget.Label)
			output := content.Objects[3].(*widget.Label)
			actions := content.Objects[4].(*fyne.Container)
			openBtn := actions.Objects[0].(*widget.Button)
			copyBtn := actions.Objects[1].(*widget.Button)

			title.SetText(job.Request.Title)
			status.SetText(fmt.Sprintf("%s • %s", job.Status, job.Stage))
			formatLabel.SetText(fmt.Sprintf("Format: %s", job.Request.Format))
			if job.Output != "" {
				output.SetText("Saved to " + job.Output)
			} else {
				output.SetText("Saved output unavailable")
			}
			if job.Output == "" {
				openBtn.Disable()
				copyBtn.Disable()
			} else {
				openBtn.Enable()
				copyBtn.Enable()
			}
			openBtn.OnTapped = func() {
				if err := OpenInFileManager(job.Output); err != nil {
					dialog.ShowError(err, w)
				}
			}
			copyBtn.OnTapped = func() {
				if job.Output == "" {
					return
				}
				w.Clipboard().SetContent(job.Output)
				statusLabel.SetText("Copied path: " + job.Output)
			}
		},
	)
	historyStack := container.NewStack(historyEmpty, historyList)

	// --- Settings Tab ---
	formatSelect := widget.NewSelect([]string{string(FormatAudioMP3), string(FormatVideoMP4), string(FormatBest)}, func(value string) {
		downloadFormat = DownloadFormat(value)
		formatHintSearch.SetText(fmt.Sprintf("Queue format: %s", downloadFormat))
		formatHintURL.SetText(fmt.Sprintf("Queue format: %s", downloadFormat))
		resultList.Refresh()
		detailFormat.SetText(fmt.Sprintf("Format: %s", downloadFormat))
		if selectedInfo != nil {
			detailSize.SetText("Estimated size: " + EstimateSize(selectedInfo, downloadFormat))
		}
	})
	formatSelect.SetSelected(string(downloadFormat))

	templateEntry := widget.NewEntry()
	templateEntry.SetText(filenameTemplate)
	templateEntry.OnChanged = func(value string) {
		filenameTemplate = value
	}

	outputLabel := widget.NewLabel("Default folder: current directory")
	outputBtn := widget.NewButtonWithIcon("Choose Folder", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			outputDir = uri.Path()
			outputLabel.SetText("Folder: " + outputDir)
		}, w)
	})

	concurrencySelect := widget.NewSelect([]string{"1", "2", "3", "4"}, func(value string) {
		next, err := strconv.Atoi(value)
		if err != nil {
			return
		}
		manager.SetMaxConcurrent(next)
	})
	concurrencySelect.SetSelected("2")

	settingsContent := container.NewVBox(
		widget.NewLabelWithStyle("Download Preferences", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Format"),
		formatSelect,
		widget.NewLabel("Output folder"),
		outputLabel,
		outputBtn,
		widget.NewLabel("Filename template"),
		templateEntry,
		widget.NewLabel("Concurrent downloads"),
		concurrencySelect,
	)
	settingsCard := NewPanel(settingsContent)

	// --- Queue Summary Panel ---
	queueSummaryList := widget.NewList(
		func() int { return len(activeItems) },
		func() fyne.CanvasObject {
			title := widget.NewLabel("Title")
			title.TextStyle = fyne.TextStyle{Bold: true}
			title.Wrapping = fyne.TextWrapWord
			progress := NewFancyProgressBar()
			status := widget.NewLabel("Downloading")
			content := container.NewVBox(title, progress, status)
			return NewPanel(content)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(activeItems) {
				return
			}
			job := activeItems[id]
			card := item.(*fyne.Container)
			padded := card.Objects[2].(*fyne.Container)
			content := padded.Objects[0].(*fyne.Container)

			title := content.Objects[0].(*widget.Label)
			progress := content.Objects[1].(*FancyProgressBar)
			status := content.Objects[2].(*widget.Label)

			title.SetText(job.Request.Title)
			progress.SetValue(job.Progress)
			progress.SetIndeterminate(job.Stage == StageConvert || job.Stage == StagePaused)
			status.SetText(string(job.Stage))
		},
	)
	queueSummaryStack := container.NewStack(queueSummaryEmpty, queueSummaryList)
	queueSummary := NewPanel(container.NewVBox(
		widget.NewLabelWithStyle("Active Queue", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		queueSummaryStack,
	))

	// --- Tabs ---
	tabs := container.NewAppTabs(
		container.NewTabItem("Search", container.NewPadded(searchSplit)),
		container.NewTabItem("Direct URL", container.NewCenter(urlCard)),
		container.NewTabItem("Queue", container.NewPadded(queueStack)),
		container.NewTabItem("History", container.NewPadded(historyStack)),
		container.NewTabItem("Settings", settingsCard),
	)
	tabs.SetTabLocation(container.TabLocationLeading)

	// --- Top Bar ---
	appTitle := widget.NewLabelWithStyle("YTDL2", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	appSubtitle := widget.NewLabel("Search, queue, and download")
	titleBlock := container.NewVBox(appTitle, appSubtitle)
	statusBlock := NewPanel(container.NewVBox(statusLabel, statusCounts))
	topBar := container.NewBorder(nil, nil, titleBlock, statusBlock)

	// --- Bottom Bar ---
	clock := widget.NewLabel("")
	clock.Alignment = fyne.TextAlignTrailing
	go func() {
		for {
			time.Sleep(time.Second)
			runOnMain(func() {
				clock.SetText(time.Now().Format("15:04:05"))
			})
		}
	}()
	bottomBar := NewPanel(container.NewHBox(widget.NewLabel("Downloads continue while the window is open"), layout.NewSpacer(), clock))

	// --- Layout ---
	content := container.NewBorder(topBar, bottomBar, nil, queueSummary, tabs)
	main := container.NewMax(NewAppBackground(), container.NewPadded(content))

	manager.SetOnChange(func() {
		if stateStore != nil {
			stateStore.Schedule(manager.SnapshotState())
		}
		runOnMain(func() {
			queueItems = manager.QueueSnapshot()
			activeItems = manager.ActiveSnapshot()
			historyItems = manager.HistorySnapshot()

			activeCount := len(activeItems)
			queuedCount := len(queueItems) - activeCount
			if queuedCount < 0 {
				queuedCount = 0
			}

			statusCounts.SetText(fmt.Sprintf("%d active • %d queued • %d history", activeCount, queuedCount, len(historyItems)))

			if len(queueItems) == 0 {
				queueEmpty.Show()
			} else {
				queueEmpty.Hide()
			}
			if len(historyItems) == 0 {
				historyEmpty.Show()
			} else {
				historyEmpty.Hide()
			}
			if len(activeItems) == 0 {
				queueSummaryEmpty.Show()
			} else {
				queueSummaryEmpty.Hide()
			}

			queueList.Refresh()
			historyList.Refresh()
			queueSummaryList.Refresh()
		})
	})

	if stateStore != nil {
		state, err := stateStore.Load()
		if err != nil {
			statusLabel.SetText("Failed to load state: " + err.Error())
		} else {
			manager.RestoreState(state)
			manager.maybeStart()
		}
	}

	w.SetContent(main)

	statusLabel.SetText("Checking dependencies...")
	go func() {
		ytDlpOK := true
		if _, err := exec.LookPath(getYtDlpPath()); err != nil {
			ytDlpOK = false
		}
		ffmpegOK := checkFFmpeg()
		time.Sleep(250 * time.Millisecond)
		runOnMain(func() {
			switch {
			case !ytDlpOK:
				statusLabel.SetText("yt-dlp not found. Search and downloads are unavailable.")
				searchBtn.Disable()
				urlAddBtn.Disable()
				detailQueueBtn.Disable()
			case !ffmpegOK:
				statusLabel.SetText("Ready. ffmpeg not found, audio conversion may fail.")
			default:
				statusLabel.SetText("Ready")
			}
		})
	}()

	w.ShowAndRun()
}
