// This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
// If a copy of the MPL was not distributed with this file, You can obtain one at
// https://mozilla.org/MPL/2.0/.

package pfte

import "sync"

// TransferJob represents a single unit of work.
type TransferJob struct {
	LocalPath  string
	RemotePath string
	Operation  string // "UPLOAD" or "DOWNLOAD"
}

// JobQueue is a thread-safe queue for transfer jobs.
// We need a Mutex because 64 goroutines will be fighting for the next job.
type JobQueue struct {
	jobs []*TransferJob
	mu   sync.Mutex
}

func NewQueue() *JobQueue {
	return &JobQueue{
		jobs: make([]*TransferJob, 0),
	}
}

// Add pushes a job to the back of the queue.
func (q *JobQueue) Add(job *TransferJob) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, job)
}

// Pop returns the next job or nil if empty.
func (q *JobQueue) Pop() *TransferJob {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.jobs) == 0 {
		return nil
	}

	// Classic queue shifting
	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	return job
}

// Count returns remaining jobs.
func (q *JobQueue) Count() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs)
}