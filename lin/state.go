package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type PersistedState struct {
	Queue   []PersistedJob `json:"queue"`
	History []PersistedJob `json:"history"`
	SavedAt time.Time      `json:"saved_at"`
}

type PersistedJob struct {
	ID        string          `json:"id"`
	Request   DownloadRequest `json:"request"`
	Status    DownloadStatus  `json:"status"`
	Stage     DownloadStage   `json:"stage"`
	Progress  float64         `json:"progress"`
	Speed     string          `json:"speed"`
	ETA       string          `json:"eta"`
	Total     string          `json:"total"`
	Message   string          `json:"message"`
	Output    string          `json:"output"`
	Error     string          `json:"error"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type StateStore struct {
	mu       sync.Mutex
	path     string
	timer    *time.Timer
	pending  PersistedState
	interval time.Duration
}

func NewStateStore(appName string) (*StateStore, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	stateDir := filepath.Join(dir, appName)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, err
	}
	return &StateStore{
		path:     filepath.Join(stateDir, "state.json"),
		interval: 1200 * time.Millisecond,
	}, nil
}

func (s *StateStore) Load() (PersistedState, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PersistedState{}, nil
		}
		return PersistedState{}, err
	}
	var state PersistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return PersistedState{}, err
	}
	return state, nil
}

func (s *StateStore) Schedule(state PersistedState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state.SavedAt = time.Now()
	s.pending = state
	if s.timer != nil {
		s.timer.Reset(s.interval)
		return
	}
	s.timer = time.AfterFunc(s.interval, func() {
		s.mu.Lock()
		pending := s.pending
		s.timer = nil
		s.mu.Unlock()
		_ = s.saveNow(pending)
	})
}

func (s *StateStore) saveNow(state PersistedState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

func (m *DownloadManager) SnapshotState() PersistedState {
	m.mu.Lock()
	defer m.mu.Unlock()

	queueJobs := make([]PersistedJob, 0, len(m.active)+len(m.queue))
	for _, job := range m.active {
		queueJobs = append(queueJobs, snapshotJob(job, true))
	}
	for _, job := range m.queue {
		queueJobs = append(queueJobs, snapshotJob(job, false))
	}

	history := make([]PersistedJob, 0, len(m.history))
	for _, job := range m.history {
		history = append(history, snapshotJob(job, false))
	}

	return PersistedState{
		Queue:   queueJobs,
		History: history,
		SavedAt: time.Now(),
	}
}

func (m *DownloadManager) RestoreState(state PersistedState) {
	m.mu.Lock()
	m.queue = nil
	m.active = nil
	m.running = 0

	for _, job := range state.Queue {
		restored := restoreJob(job)
		m.queue = append(m.queue, restored)
	}
	for _, job := range state.History {
		restored := restoreJob(job)
		m.history = append(m.history, restored)
	}
	m.mu.Unlock()
	m.notify()
}

func snapshotJob(job *DownloadJob, wasActive bool) PersistedJob {
	status := job.Status
	stage := job.Stage
	if wasActive || status == StatusRunning || status == StatusPaused {
		status = StatusQueued
		stage = StageQueued
	}
	return PersistedJob{
		ID:        job.ID,
		Request:   job.Request,
		Status:    status,
		Stage:     stage,
		Progress:  job.Progress,
		Speed:     job.Speed,
		ETA:       job.ETA,
		Total:     job.Total,
		Message:   job.Message,
		Output:    job.Output,
		Error:     job.Error,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
	}
}

func restoreJob(job PersistedJob) *DownloadJob {
	status := job.Status
	stage := job.Stage
	if status == StatusRunning || status == StatusPaused {
		status = StatusQueued
		stage = StageQueued
	}
	return &DownloadJob{
		ID:        job.ID,
		Request:   job.Request,
		Status:    status,
		Stage:     stage,
		Progress:  job.Progress,
		Speed:     job.Speed,
		ETA:       job.ETA,
		Total:     job.Total,
		Message:   job.Message,
		Output:    job.Output,
		Error:     job.Error,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
	}
}
