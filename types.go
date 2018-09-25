package commander

import (
	"context"
	"fmt"
	"time"
)

type QuitError struct{}

func (QuitError) Error() string {
	return "quit"
}

type Config interface{}

type NameFunc func() string
type ExecuteFunc func(*Config, interface{}) (interface{}, error)
type PayloadFunc func(*Config) (interface{}, error)
type AdditionsFunc func(*Config) map[string]Action
type RemovalsFunc func() []string
type TagsFunc func() []string

type Name interface {
	Name() string
}

type Work struct {
	Name    string
	job     ExecuteFunc
	payload interface{}
	wait    chan struct{}
	// job     func(*Config, interface{}) (interface{}, error)
	// only populated after Do is called on the result
	Result     interface{}
	FinishedAt time.Time
	CreatedAt  time.Time
}

func WorkFromAction(a Action, payload interface{}) *Work {
	return &Work{
		Name:      a.Name(),
		job:       a.Execute,
		wait:      make(chan struct{}, 1),
		CreatedAt: time.Now(),
	}
}

func (w *Work) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-w.wait:
		return nil
	}
}

func (w *Work) do(conf *Config) error {
	defer func() {
		close(w.wait)
	}()

	res, err := w.job(conf, w.payload)
	w.FinishedAt = time.Now()
	if err != nil {
		w.Result = err
		return err
	}
	w.Result = res

	return nil
}

type WorkChan struct {
	CachedResults map[string]*Work
	queue         chan *Work
}

func NewWorkChan(buff int64) *WorkChan {
	return &WorkChan{
		CachedResults: make(map[string]*Work),
		queue:         make(chan *Work, buff),
	}
}
func (w *WorkChan) Start(conf *Config) {
	for v := range w.queue {
		w.CachedResults[v.Name] = v
		v.do(conf)
	}
}
func (w *WorkChan) Stop() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("channel was probably closed already: %v\n", r)
		}
	}()
	close(w.queue)
}
func (w *WorkChan) Queue(work *Work) {
	w.queue <- work
}
func (w *WorkChan) Dequeue() *Work {
	return <-w.queue
}
