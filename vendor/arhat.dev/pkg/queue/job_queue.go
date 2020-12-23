// +build !noqueue_jobqueue

/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package queue

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// Errors for JobQueue
var (
	ErrJobDuplicated = errors.New("job duplicat")
	ErrJobConflict   = errors.New("job conflict")
	ErrJobCounteract = errors.New("job counteract")
	ErrJobInvalid    = errors.New("job invalid")
)

type JobAction uint8

const (
	// ActionInvalid to do nothing
	ActionInvalid JobAction = iota
	// ActionAdd to add or create some resource
	ActionAdd
	// ActionUpdate to update some resource
	ActionUpdate
	// ActionDelete to delete some resource
	ActionDelete
	// ActionCleanup to eliminate all side effects of the resource
	ActionCleanup
)

var actionNames = map[JobAction]string{
	ActionInvalid: "Invalid",
	ActionAdd:     "Add",
	ActionUpdate:  "Update",
	ActionDelete:  "Delete",
	ActionCleanup: "Cleanup",
}

func (t JobAction) String() string {
	return actionNames[t]
}

// Job item to record action and related resource object
type Job struct {
	Action JobAction
	Key    interface{}
}

func (w Job) String() string {
	if s, ok := w.Key.(fmt.Stringer); ok {
		return w.Action.String() + "/" + s.String()
	}

	return fmt.Sprintf("%s/%v", w.Action.String(), w.Key)
}

// NewJobQueue will create a stopped new job queue,
// you can offer job to it, but any acquire will fail until
// you have called its Resume()
func NewJobQueue() *JobQueue {
	// prepare a closed channel for this job queue
	hasJob := make(chan struct{})
	close(hasJob)

	return &JobQueue{
		queue: make([]Job, 0, 16),
		index: make(map[Job]int),

		// set job queue to closed
		hasJob:     hasJob,
		chanClosed: true,
		mu:         new(sync.RWMutex),

		paused: 1,
	}
}

// JobQueue is the queue data structure designed to reduce redundant job
// as much as possible
type JobQueue struct {
	queue []Job
	index map[Job]int

	hasJob chan struct{}
	mu     *sync.RWMutex
	// protected by atomic
	paused     uint32
	chanClosed bool
}

func (q *JobQueue) has(action JobAction, key interface{}) bool {
	_, ok := q.index[Job{Action: action, Key: key}]
	return ok
}

func (q *JobQueue) add(w Job) {
	q.index[w] = len(q.queue)
	q.queue = append(q.queue, w)
}

func (q *JobQueue) delete(action JobAction, key interface{}) bool {
	jobToDelete := Job{Action: action, Key: key}
	if idx, ok := q.index[jobToDelete]; ok {
		delete(q.index, jobToDelete)
		q.queue = append(q.queue[:idx], q.queue[idx+1:]...)

		q.buildIndex()

		return true
	}

	return false
}

func (q *JobQueue) buildIndex() {
	for i, w := range q.queue {
		q.index[w] = i
	}
}

// Remains shows what job we are still meant to do
func (q *JobQueue) Remains() []Job {
	q.mu.RLock()
	defer q.mu.RUnlock()

	jobs := make([]Job, len(q.queue))
	for i, w := range q.queue {
		jobs[i] = Job{Action: w.Action, Key: w.Key}
	}
	return jobs
}

// Find the scheduled job according to its key
func (q *JobQueue) Find(key interface{}) (Job, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for _, t := range []JobAction{ActionAdd, ActionUpdate, ActionDelete, ActionCleanup} {
		i, ok := q.index[Job{Action: t, Key: key}]
		if ok {
			return q.queue[i], true
		}
	}

	return Job{}, false
}

// Acquire a job item from the job queue
// if shouldAcquireMore is false, w will be an empty job
func (q *JobQueue) Acquire() (w Job, shouldAcquireMore bool) {
	// wait until we have got some job to do
	// or we have paused the job queue
	<-q.hasJob

	if q.isPaused() {
		return Job{Action: ActionInvalid}, false
	}

	q.mu.Lock()
	defer func() {
		if len(q.queue) == 0 {
			if !q.isPaused() {
				q.hasJob = make(chan struct{})
				q.chanClosed = false
			}
		}

		q.mu.Unlock()
	}()

	if len(q.queue) == 0 {
		return Job{Action: ActionInvalid}, true
	}

	// pop first and rebuild index
	w = q.queue[0]
	q.delete(w.Action, w.Key)

	return w, true
}

// Offer a job item to the job queue
// if offered job was not added, an error result will return, otherwise nil
func (q *JobQueue) Offer(w Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if w.Action == ActionInvalid {
		return ErrJobInvalid
	}

	_, dup := q.index[w]
	if dup {
		return ErrJobDuplicated
	}

	switch w.Action {
	case ActionAdd:
		if q.has(ActionUpdate, w.Key) {
			return ErrJobConflict
		}

		q.add(w)
	case ActionUpdate:
		if q.has(ActionAdd, w.Key) || q.has(ActionDelete, w.Key) {
			return ErrJobConflict
		}

		q.add(w)
	case ActionDelete:
		// pod need to be deleted
		if q.has(ActionAdd, w.Key) {
			// cancel according create job
			q.delete(ActionAdd, w.Key)
			return ErrJobCounteract
		}

		if q.has(ActionUpdate, w.Key) {
			// if you want to delete it now, update operation doesn't matter any more
			q.delete(ActionUpdate, w.Key)
		}

		q.add(w)
	case ActionCleanup:
		// cleanup job only requires no duplication

		q.add(w)
	}

	// we reach here means we have added some job to the queue
	// we should signal those consumers to go for it
	select {
	case <-q.hasJob:
		// we can reach here means q.hasJob has been closed
	default:
		// release the signal
		close(q.hasJob)
		// mark the channel closed to prevent a second close which would panic
		q.chanClosed = true
	}

	return nil
}

// Resume do nothing but mark you can perform acquire
// actions to the job queue
func (q *JobQueue) Resume() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.chanClosed && len(q.queue) == 0 {
		// reopen signal channel for wait
		q.hasJob = make(chan struct{})
		q.chanClosed = false
	}

	atomic.StoreUint32(&q.paused, 0)
}

// Pause do nothing but mark this job queue is closed,
// you should not perform acquire actions to the job queue
func (q *JobQueue) Pause() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.chanClosed {
		// close wait channel to prevent wait
		close(q.hasJob)
		q.chanClosed = true
	}

	atomic.StoreUint32(&q.paused, 1)
}

func (q *JobQueue) Remove(w Job) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.delete(w.Action, w.Key)
}

// isPaused is just for approximate check, for real
// closed state, need to hold the lock
func (q *JobQueue) isPaused() bool {
	return atomic.LoadUint32(&q.paused) == 1
}
