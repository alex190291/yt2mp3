package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DownloadFormat string

const (
	FormatAudioMP3 DownloadFormat = "Audio (MP3)"
	FormatVideoMP4 DownloadFormat = "Video (MP4)"
	FormatBest     DownloadFormat = "Best Available"
)

type DownloadRequest struct {
	URL              string
	Title            string
	Format           DownloadFormat
	OutputDir        string
	FilenameTemplate string
}

type DownloadStatus string

const (
	StatusQueued    DownloadStatus = "Queued"
	StatusRunning   DownloadStatus = "Running"
	StatusPaused    DownloadStatus = "Paused"
	StatusCompleted DownloadStatus = "Completed"
	StatusFailed    DownloadStatus = "Failed"
	StatusCanceled  DownloadStatus = "Canceled"
)

type DownloadStage string

const (
	StageQueued    DownloadStage = "Queued"
	StageDownload  DownloadStage = "Downloading"
	StageConvert   DownloadStage = "Converting"
	StagePaused    DownloadStage = "Paused"
	StageCompleted DownloadStage = "Completed"
	StageFailed    DownloadStage = "Failed"
	StageCanceled  DownloadStage = "Canceled"
)

type DownloadJob struct {
	ID        string
	Request   DownloadRequest
	Status    DownloadStatus
	Stage     DownloadStage
	prevStage DownloadStage
	Progress  float64
	Speed     string
	ETA       string
	Total     string
	Message   string
	Output    string
	Error     string
	CreatedAt time.Time
	UpdatedAt time.Time

	cancel context.CancelFunc
	cmd    *exec.Cmd
}

type DownloadManager struct {
	mu            sync.Mutex
	queue         []*DownloadJob
	active        []*DownloadJob
	history       []*DownloadJob
	running       int
	maxConcurrent int
	onChange      func()
}

func NewDownloadManager(maxConcurrent int, onChange func()) *DownloadManager {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &DownloadManager{
		maxConcurrent: maxConcurrent,
		onChange:      onChange,
	}
}

func (m *DownloadManager) SetOnChange(fn func()) {
	m.mu.Lock()
	m.onChange = fn
	m.mu.Unlock()
}

func (m *DownloadManager) SetMaxConcurrent(n int) {
	if n < 1 {
		n = 1
	}
	m.mu.Lock()
	m.maxConcurrent = n
	m.mu.Unlock()
	m.maybeStart()
}

func (m *DownloadManager) Add(req DownloadRequest) *DownloadJob {
	job := &DownloadJob{
		ID:        fmt.Sprintf("job-%d", time.Now().UnixNano()),
		Request:   req,
		Status:    StatusQueued,
		Stage:     StageQueued,
		Progress:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if job.Request.Title == "" {
		job.Request.Title = job.Request.URL
	}

	m.mu.Lock()
	m.queue = append(m.queue, job)
	m.mu.Unlock()
	m.notify()
	m.maybeStart()
	return job
}

func (m *DownloadManager) Cancel(id string) {
	var canceled *DownloadJob
	m.mu.Lock()
	for i, job := range m.queue {
		if job.ID == id {
			canceled = job
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			break
		}
	}
	if canceled != nil {
		canceled.Status = StatusCanceled
		canceled.Stage = StageCanceled
		canceled.UpdatedAt = time.Now()
		m.history = append([]*DownloadJob{canceled}, m.history...)
		m.mu.Unlock()
		m.notify()
		return
	}

	for _, job := range m.active {
		if job.ID == id {
			job.Status = StatusCanceled
			job.Stage = StageCanceled
			job.UpdatedAt = time.Now()
			if job.cancel != nil {
				job.cancel()
			}
			break
		}
	}
	m.mu.Unlock()
	m.notify()
}

func (m *DownloadManager) Pause(id string) error {
	job := m.findActiveJob(id)
	if job == nil {
		return errors.New("job not found")
	}
	if job.Status == StatusPaused {
		return nil
	}
	if job.cmd == nil || job.cmd.Process == nil {
		return errors.New("process not available")
	}
	if err := pauseProcess(job.cmd.Process); err != nil {
		return err
	}

	m.mu.Lock()
	job.prevStage = job.Stage
	job.Status = StatusPaused
	job.Stage = StagePaused
	job.Message = "Paused"
	job.UpdatedAt = time.Now()
	m.mu.Unlock()
	m.notify()
	return nil
}

func (m *DownloadManager) Resume(id string) error {
	job := m.findActiveJob(id)
	if job == nil {
		return errors.New("job not found")
	}
	if job.Status != StatusPaused {
		return nil
	}
	if job.cmd == nil || job.cmd.Process == nil {
		return errors.New("process not available")
	}
	if err := resumeProcess(job.cmd.Process); err != nil {
		return err
	}

	m.mu.Lock()
	job.Status = StatusRunning
	if job.prevStage != "" && job.prevStage != StagePaused {
		job.Stage = job.prevStage
	} else {
		job.Stage = StageDownload
	}
	job.Message = "Resumed"
	job.UpdatedAt = time.Now()
	m.mu.Unlock()
	m.notify()
	return nil
}

func (m *DownloadManager) QueueSnapshot() []*DownloadJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	all := make([]*DownloadJob, 0, len(m.active)+len(m.queue))
	all = append(all, m.active...)
	all = append(all, m.queue...)
	return all
}

func (m *DownloadManager) ActiveSnapshot() []*DownloadJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	active := make([]*DownloadJob, len(m.active))
	copy(active, m.active)
	return active
}

func (m *DownloadManager) HistorySnapshot() []*DownloadJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	history := make([]*DownloadJob, len(m.history))
	copy(history, m.history)
	return history
}

func (m *DownloadManager) maybeStart() {
	notify := false
	m.mu.Lock()
	for m.running < m.maxConcurrent && len(m.queue) > 0 {
		job := m.queue[0]
		m.queue = m.queue[1:]
		job.Status = StatusRunning
		job.Stage = StageDownload
		job.UpdatedAt = time.Now()
		m.active = append(m.active, job)
		m.running++
		notify = true
		go m.runJob(job)
	}
	m.mu.Unlock()
	if notify {
		m.notify()
	}
}

func (m *DownloadManager) runJob(job *DownloadJob) {
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	job.cancel = cancel
	m.mu.Unlock()

	cmd := Download(ctx, job.Request)
	m.mu.Lock()
	job.cmd = cmd
	m.mu.Unlock()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.failJob(job, err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.failJob(job, err)
		return
	}
	if err := cmd.Start(); err != nil {
		m.failJob(job, err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		m.scanLines(job, stdout)
		wg.Done()
	}()
	go func() {
		m.scanLines(job, stderr)
		wg.Done()
	}()

	waitErr := cmd.Wait()
	wg.Wait()

	m.mu.Lock()
	if job.Status == StatusCanceled || errors.Is(waitErr, context.Canceled) {
		job.Status = StatusCanceled
		job.Stage = StageCanceled
	} else if waitErr != nil {
		job.Status = StatusFailed
		job.Stage = StageFailed
		job.Error = waitErr.Error()
	} else {
		job.Status = StatusCompleted
		job.Stage = StageCompleted
		job.Progress = 1
	}
	job.UpdatedAt = time.Now()
	m.mu.Unlock()

	m.finishJob(job)
}

func (m *DownloadManager) scanLines(job *DownloadJob, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		m.handleLine(job, line)
	}
}

func (m *DownloadManager) handleLine(job *DownloadJob, line string) {
	m.mu.Lock()
	if job.Status == StatusPaused {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	if dest, ok := parseDestination(line); ok {
		m.mu.Lock()
		job.Output = dest
		job.Message = "Saving to " + dest
		job.UpdatedAt = time.Now()
		m.mu.Unlock()
		m.notify()
		return
	}

	if strings.Contains(line, "ExtractAudio") || strings.Contains(line, "ffmpeg") || strings.Contains(line, "Post-process") {
		m.mu.Lock()
		job.Stage = StageConvert
		job.Message = "Converting..."
		job.UpdatedAt = time.Now()
		m.mu.Unlock()
		m.notify()
		return
	}

	if percent, total, speed, eta, ok := parseProgressLine(line); ok {
		m.mu.Lock()
		job.Stage = StageDownload
		job.Progress = percent / 100
		job.Total = total
		job.Speed = speed
		job.ETA = eta
		job.Message = fmt.Sprintf("%0.1f%% of %s", percent, total)
		job.UpdatedAt = time.Now()
		m.mu.Unlock()
		m.notify()
		return
	}

	if strings.Contains(strings.ToLower(line), "error:") {
		m.mu.Lock()
		job.Error = line
		job.UpdatedAt = time.Now()
		m.mu.Unlock()
		m.notify()
	}
}

func (m *DownloadManager) finishJob(job *DownloadJob) {
	m.mu.Lock()
	for i, activeJob := range m.active {
		if activeJob.ID == job.ID {
			m.active = append(m.active[:i], m.active[i+1:]...)
			break
		}
	}
	if m.running > 0 {
		m.running--
	}
	m.history = append([]*DownloadJob{job}, m.history...)
	m.mu.Unlock()
	m.notify()
	m.maybeStart()
}

func (m *DownloadManager) failJob(job *DownloadJob, err error) {
	m.mu.Lock()
	job.Status = StatusFailed
	job.Stage = StageFailed
	job.Error = err.Error()
	job.UpdatedAt = time.Now()
	m.mu.Unlock()
	m.finishJob(job)
}

func (m *DownloadManager) notify() {
	m.mu.Lock()
	onChange := m.onChange
	m.mu.Unlock()
	if onChange != nil {
		onChange()
	}
}

func (m *DownloadManager) findActiveJob(id string) *DownloadJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, job := range m.active {
		if job.ID == id {
			return job
		}
	}
	return nil
}

var (
	progressRe  = regexp.MustCompile(`^\[download\]\s+(\d+(?:\.\d+)?)% of\s+~?([^\s]+)\s+at\s+([^\s]+)\s+ETA\s+([^\s]+)`)
	progressRe2 = regexp.MustCompile(`^\[download\]\s+(\d+(?:\.\d+)?)% of\s+~?([^\s]+)\s+in\s+([^\s]+)`)
	progressRe3 = regexp.MustCompile(`^\[download\]\s+(\d+(?:\.\d+)?)% of\s+~?([^\s]+)`)
	destRe      = regexp.MustCompile(`Destination:\s+(.+)$`)
)

func parseProgressLine(line string) (percent float64, total, speed, eta string, ok bool) {
	if matches := progressRe.FindStringSubmatch(line); matches != nil {
		percent, _ = strconv.ParseFloat(matches[1], 64)
		total = matches[2]
		speed = matches[3]
		eta = matches[4]
		return percent, total, speed, eta, true
	}
	if matches := progressRe2.FindStringSubmatch(line); matches != nil {
		percent, _ = strconv.ParseFloat(matches[1], 64)
		total = matches[2]
		speed = "done in " + matches[3]
		return percent, total, speed, "", true
	}
	if matches := progressRe3.FindStringSubmatch(line); matches != nil {
		percent, _ = strconv.ParseFloat(matches[1], 64)
		total = matches[2]
		return percent, total, "", "", true
	}
	return 0, "", "", "", false
}

func parseDestination(line string) (string, bool) {
	if matches := destRe.FindStringSubmatch(line); matches != nil {
		return strings.TrimSpace(matches[1]), true
	}
	return "", false
}
