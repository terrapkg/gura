package util

import (
	"container/heap"
	"sync"
	"time"
)

// Scheduler allows scheduling tasks to run at specific times using a single goroutine.
// It is safe for concurrent use.
type Scheduler struct {
	mu      sync.Mutex
	cond    *sync.Cond
	tasks   taskHeap
	stopped bool
	wake    chan struct{}
}

// NewScheduler creates and starts a new Scheduler.
func NewScheduler() *Scheduler {
	s := &Scheduler{
		tasks: make(taskHeap, 0),
		wake:  make(chan struct{}, 1),
	}
	s.cond = sync.NewCond(&s.mu)
	go s.loop()
	return s
}

// Schedule schedules a function to run at the specified time.
// If the time is in the past, the function will run as soon as possible.
func (s *Scheduler) Schedule(at time.Time, fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	heap.Push(&s.tasks, &scheduledTask{at: at, fn: fn})
	// If this is the earliest task, wake up the scheduler.
	if s.tasks[0].at.Equal(at) {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

// Stop stops the scheduler and prevents any further tasks from running.
// Already running tasks may still complete.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Scheduler) loop() {
	for {
		s.mu.Lock()
		for len(s.tasks) == 0 && !s.stopped {
			s.mu.Unlock()
			<-s.wake // Wait for a task or stop signal
			s.mu.Lock()
		}
		if s.stopped {
			s.mu.Unlock()
			return
		}
		now := time.Now()
		task := s.tasks[0]
		if task.at.After(now) {
			wait := task.at.Sub(now)
			s.mu.Unlock()
			select {
			case <-time.After(wait):
			case <-s.wake:
			}
			continue
		}
		heap.Pop(&s.tasks)
		s.mu.Unlock()
		// Run the task outside the lock
		go task.fn()
	}
}

// scheduledTask represents a scheduled function and its run time.
type scheduledTask struct {
	at time.Time
	fn func()
}

// taskHeap implements heap.Interface for scheduledTask.
type taskHeap []*scheduledTask

func (h taskHeap) Len() int           { return len(h) }
func (h taskHeap) Less(i, j int) bool { return h[i].at.Before(h[j].at) }
func (h taskHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *taskHeap) Push(x any) {
	*h = append(*h, x.(*scheduledTask))
}

func (h *taskHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}
