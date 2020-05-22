// Package workpool for do task in work pool.
package workpool

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Task task struct.
type Task struct {
	fn func() error
}

// NewTask returns task,create a task entry.
func NewTask(fn func() error) *Task {
	return &Task{
		fn: fn,
	}
}

// run exec a task.
func (t *Task) run(logEntry logger) {
	defer func() {
		if e := recover(); e != nil {
			logEntry.Println("exec current task panic: ", e)
		}
	}()

	t.fn()
}

// logger log record interface
type logger interface {
	Println(args ...interface{})
}

// Pool task work pool
type Pool struct {
	EntryChan chan *Task
	jobChan   chan *Task
	workerNum int            // worker num
	logEntry  logger         // logger interface
	interrupt chan os.Signal // exit signal,ctrl +c to interrupt work pool
	stop      chan bool      // shutdown stop signal
	wait      time.Duration  // graceful shutdown to wait time
}

// defaultMaxEntryCap default max entry chan num.
var defaultMaxEntryCap = 10000

// defaultMaxJobCap default max job chan cap.
var defaultMaxJobCap = 10000

// Option func Option to change pool
type Option func(p *Pool)

// NewPool returns a pool.
func NewPool(num int, ec int, jc ...int) *Pool {
	if ec < 0 || ec >= defaultMaxEntryCap {
		ec = defaultMaxEntryCap
	}

	// jobChan buf num
	var jobChanCap int
	if len(jc) > 0 && jc[0] > 0 {
		jobChanCap = jc[0]
	}

	if jobChanCap < 0 || jobChanCap >= defaultMaxJobCap {
		jobChanCap = defaultMaxJobCap
	}

	p := &Pool{
		workerNum: num,
		stop:      make(chan bool, 1),
		interrupt: make(chan os.Signal, 1),
		wait:      5 * time.Second,
	}

	if jobChanCap == 0 {
		// no buf for jobChan.
		p.jobChan = make(chan *Task)
	} else {
		p.jobChan = make(chan *Task, ec)
	}

	if ec == 0 {
		// no buf for EntryChan.
		p.EntryChan = make(chan *Task)
	} else {
		p.EntryChan = make(chan *Task, ec)
	}

	if p.logEntry == nil {
		p.logEntry = log.New(os.Stderr, "", log.LstdFlags)
	}

	return p
}

// WithLogger change logger entry.
func WithLogger(logEntry logger) Option {
	return func(p *Pool) {
		p.logEntry = logEntry
	}
}

// WithWaitTime task pool shutdown to wait time.
func WithWaitTime(d time.Duration) Option {
	return func(p *Pool) {
		p.wait = d
	}
}

// AddTask add a task to p.EntryChan.
func (p *Pool) AddTask(t *Task) {
	select {
	case <-p.stop:
	default:
		p.EntryChan <- t
	}
}

// BatchAddTask batch add task to p.EntryChan.
func (p *Pool) BatchAddTask(t []*Task) {
	for k := range t {
		select {
		case <-p.stop:
			return
		default:
			p.EntryChan <- t[k]
		}
	}
}

// exec exec task from job chan.
func (p *Pool) exec(id int) {
	defer p.recovery()

	defer func() {
		p.logEntry.Println("current worker id: ", id, "will exit...")
	}()

	// get task from JobChan to run.
	for task := range p.jobChan {
		task.run(p.logEntry)
		p.logEntry.Println("current worker id: ", id)
	}
}

// Run create workerNum goroutine to exec task.
func (p *Pool) Run() {
	p.logEntry.Println("exec task begin...")
	signal.Notify(p.interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR2, os.Interrupt, syscall.SIGHUP)

	// create p.workerNum goroutine to do task
	for i := 0; i < p.workerNum; i++ {
		go p.exec(i)
	}

	// listen interrupt signal.
	go func() {
		for {
			if p.isInterrupt() {
				p.stop <- true

				p.logEntry.Println("task will exit...")

				// Create a deadline to wait for all task graceful exit.
				ctx, cancel := context.WithTimeout(context.Background(), p.wait)
				defer cancel()

				// Doesn't block if no task, but will otherwise wait
				// until the timeout deadline.
				// Optionally, you could run shutdown in a goroutine and block on
				// if your application should wait for all task to run
				// to finalize based on context cancellation.
				<-ctx.Done()

				// close entry chan and job chan to graceful shutdown.
				close(p.EntryChan)
				close(p.jobChan)

				break
			}
		}
	}()

	// throw task to JobChan
	for {
		select {
		case p.jobChan <- <-p.EntryChan:
		case <-p.stop:
			return
		}
	}
}

// isInterrupt return true,if receive a signal to interrupt task exec.
func (p *Pool) isInterrupt() bool {
	select {
	case sg := <-p.interrupt:
		p.logEntry.Println("received signal: ", sg.String())
		return true
	default:
		return false
	}
}

// recovery catch a recover.
func (p *Pool) recovery() {
	defer func() {
		if e := recover(); e != nil {
			p.logEntry.Println("exec panic: ", e)
		}
	}()
}
