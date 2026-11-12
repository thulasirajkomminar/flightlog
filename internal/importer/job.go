package importer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// jobRetention is how long a finished job stays available for status checks.
const jobRetention = time.Hour

// ErrImportRunning is returned when a user starts an import while their
// previous one is still in progress.
var ErrImportRunning = errors.New("an import is already running")

// JobState is the lifecycle state of a background import.
type JobState string

// Background import states.
const (
	JobRunning   JobState = "running"
	JobCompleted JobState = "completed"
)

// Job is a snapshot of a user's background import.
type Job struct {
	State     JobState      `json:"state"`
	Total     int           `json:"total"`
	Processed int           `json:"processed"`
	Result    *ImportResult `json:"result,omitempty"`

	finishedAt time.Time
}

// jobStore keeps the latest import job per user in memory. Jobs do not
// survive a restart; flights imported before the restart are kept.
type jobStore struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

func newJobStore() *jobStore {
	return &jobStore{jobs: make(map[string]*Job)}
}

// start registers a new running job for the user, replacing a finished one.
func (s *jobStore) start(userID string, total int, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, job := range s.jobs {
		if job.State == JobCompleted && now.Sub(job.finishedAt) > jobRetention {
			delete(s.jobs, id)
		}
	}

	if job, ok := s.jobs[userID]; ok && job.State == JobRunning {
		return ErrImportRunning
	}

	s.jobs[userID] = &Job{State: JobRunning, Total: total}

	return nil
}

func (s *jobStore) progress(userID string, processed int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job, ok := s.jobs[userID]; ok {
		job.Processed = processed
	}
}

func (s *jobStore) finish(userID string, result *ImportResult, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job, ok := s.jobs[userID]; ok {
		job.State = JobCompleted
		job.Processed = job.Total
		job.Result = result
		job.finishedAt = now
	}
}

// get returns a copy of the user's latest job.
func (s *jobStore) get(userID string) (Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[userID]
	if !ok {
		return Job{}, false
	}

	return *job, true
}

// StartImport begins importing entries in the background and returns the new
// job. Progress is available through ImportStatus. ctx must outlive the
// request that started the import.
func (s *Service) StartImport(ctx context.Context, userID string, entries []ImportEntry, source string, enrich bool) (Job, error) {
	err := s.jobs.start(userID, len(entries), time.Now())
	if err != nil {
		return Job{}, err
	}

	go s.runJob(ctx, userID, entries, source, enrich)

	job, _ := s.jobs.get(userID)

	return job, nil
}

// ImportStatus returns the user's running or most recently finished import.
func (s *Service) ImportStatus(userID string) (Job, bool) {
	return s.jobs.get(userID)
}

func (s *Service) runJob(ctx context.Context, userID string, entries []ImportEntry, source string, enrich bool) {
	// A panic here would take the whole server down; fail the job instead.
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("import job panicked", fmt.Errorf("%w: %v", errJobPanic, r), zap.String("user_id", userID))
			s.jobs.finish(userID, &ImportResult{Total: len(entries), Failed: len(entries)}, time.Now())
		}
	}()

	result := s.run(ctx, userID, entries, source, enrich, func(processed int) {
		s.jobs.progress(userID, processed)
	})

	s.jobs.finish(userID, result, time.Now())
}

var errJobPanic = errors.New("panic")
