package reconcile

import (
	"context"
	"errors"
	"time"

	"arhat.dev/pkg/backoff"
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
)

func NewCore(ctx context.Context, resolvedOpts *Options) *Core {
	return &Core{
		ctx: ctx,
		log: resolvedOpts.Logger,

		jobQ: queue.NewJobQueue(),

		Cache: NewCache(),

		requireCache: resolvedOpts.RequireCache,
		scheduleQ:    queue.NewTimeoutQueue(),
		backoff:      resolvedOpts.BackoffStrategy,

		h: resolvedOpts.Handlers.ResolveNil(),

		onBackoffStart: resolvedOpts.OnBackoffStart,
		onBackoffReset: resolvedOpts.OnBackoffReset,
	}
}

type Core struct {
	ctx context.Context
	log log.Interface

	jobQ *queue.JobQueue

	*Cache

	requireCache bool
	scheduleQ    *queue.TimeoutQueue
	backoff      *backoff.Strategy

	h *HandleFuncs

	onBackoffStart BackoffStartCallback
	onBackoffReset BackoffResetCallback
}

func (c *Core) Start() error {
	c.scheduleQ.Start(c.ctx.Done())

	go func() {
		for t := range c.scheduleQ.TakeCh() {
			job := t.Key.(queue.Job)

			err := c.jobQ.Offer(job)
			if err != nil && !errors.Is(err, queue.ErrJobDuplicated) {
				c.log.V("failed to schedule", log.Any("job", job), log.Error(err))
			}
		}
	}()

	return nil
}

func (c *Core) ReconcileUntil(stop <-chan struct{}) {
	c.jobQ.Resume()
	defer c.jobQ.Pause()

	go func() {
		for {
			job, more := c.jobQ.Acquire()
			if !more {
				return
			}

			c.handleJob(job)
		}
	}()

	select {
	case <-c.ctx.Done():
		return
	case <-stop:
		return
	}
}

func (c *Core) Schedule(job queue.Job, delay time.Duration) error {
	// make sure all on going jobs are unique
	_ = c.CancelSchedule(job)

	if delay == 0 {
		err := c.jobQ.Offer(job)
		if err != nil && !errors.Is(err, queue.ErrJobDuplicated) {
			return err
		}
	} else {
		return c.scheduleQ.OfferWithDelay(job, nil, delay)
	}

	return nil
}

func (c *Core) CancelSchedule(job queue.Job) bool {
	removedFromJobQ := c.jobQ.Remove(job)
	_, removedFromScheduleQ := c.scheduleQ.Remove(job)

	return removedFromJobQ || removedFromScheduleQ
}

func (c *Core) handleJob(job queue.Job) {
	var (
		result *Result
		logger = c.log.WithFields(log.Any("job", job.String()))
	)

	if job.Action == queue.ActionInvalid {
		return
	}

	previous, current := c.Get(job.Key)

	if c.requireCache && (previous == nil || current == nil) {
		result = resultCacheNotFound
		goto handleResult
	}

	switch job.Action {
	case queue.ActionAdd:
		result = c.h.OnAdded(current)
	case queue.ActionUpdate:
		result = c.h.OnUpdated(previous, current)
		if result == nil || result.Err == nil {
			// updated successfully, no need to keep old cache any more
			c.Freeze(job.Key, false)
		}
	case queue.ActionDelete:
		result = c.h.OnDeleting(current)
	case queue.ActionCleanup:
		result = c.h.OnDeleted(current)
		if result == nil || result.NextAction == queue.ActionInvalid {
			// no further action for this key, check pending jobs with same key
			_, hasPendingJob := c.jobQ.Find(job.Key)
			if !hasPendingJob {
				// no pending job with this key
				logger.V("deleting cache")
				c.Delete(job.Key)
			}
		}
	default:
		logger.V("unknown action")
		return
	}

	if result == nil {
		return
	}

handleResult:
	nA := result.NextAction
	delay := result.ScheduleAfter
	if result.Err != nil {
		nA = job.Action
		if delay == 0 {
			delay = c.backoff.Next(job.Key)
		}

		if delay != 0 && c.onBackoffStart != nil {
			c.onBackoffStart(job.Key, result.Err)
		}
	} else if c.backoff.Reset(job.Key) && c.onBackoffReset != nil {
		c.onBackoffReset(job.Key)
	}

	if nA == queue.ActionInvalid {
		return
	}

	nextJob := queue.Job{Action: nA, Key: job.Key}
	logger = logger.WithFields(log.Any("nextJob", nextJob))
	if delay > 0 {
		logger.V("scheduling next job with delay", log.Duration("delay", delay))
		err := c.scheduleQ.OfferWithDelay(nextJob, nil, delay)
		if err != nil {
			logger.V("failed to reschedule job with delay", log.Error(err))
		}
	} else {
		logger.V("scheduling next job immediately")
		err := c.jobQ.Offer(nextJob)
		if err != nil {
			logger.V("failed to schedule next job", log.Error(err))
		}
	}
}
