package commander

import (
	"context"
	"fmt"
	"time"
)

// the error type returned whenever Commands.Get("quit")() is called
type QuitError struct{}

func (QuitError) Error() string {
	return "quit"
}

// Could be anything, but helps distinguish between payload
// which is less significant
type Config interface{}

// The function signiture of Action.Name function
type NameFunc func() string

// The function signiture of Action.Desc function
type DescFunc func() string

// The function signiture of the Action.Execute function
type ExecuteFunc func(*Config, interface{}) (interface{}, error)

// ExecuteFunc's can be merged togther.
// var ein interface{}
// var e ExecuteFunc
// var f ExecuteFunc
// g := e.Chain(f)
// g(in)
// // is the same as
// if eout, err := e(ein); err == nil { f(eout) }
//
func (e ExecuteFunc) Chain(f ExecuteFunc) ExecuteFunc {
	return func(conf *Config, payload interface{}) (interface{}, error) {
		res, err := e(conf, payload)
		if err == nil {
			return f(conf, res)
		}
		return nil, err
	}
}

// the function signiture of the Action.Payload function
type PayloadFunc func(*Config) (interface{}, error)

// the function signiture of the Action.Addtions function
type AdditionsFunc func(*Config) map[string]Action

// the function signiture of the Action.Removals function
type RemovalsFunc func() []string

// the function signiture of the Action.Tags function
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

func workFromAction(a Action, payload interface{}) *Work {
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

type workChan struct {
	CachedResults map[string]*Work
	queue         chan *Work
}

func newWorkChan(buff int64) *workChan {
	return &workChan{
		CachedResults: make(map[string]*Work),
		queue:         make(chan *Work, buff),
	}
}
func (w *workChan) Start(conf *Config) {
	for v := range w.queue {
		w.CachedResults[v.Name] = v
		v.do(conf)
	}
}
func (w *workChan) Stop() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("channel was probably closed already: %v\n", r)
		}
	}()
	close(w.queue)
}
func (w *workChan) Queue(work *Work) {
	w.queue <- work
}
func (w *workChan) Dequeue() *Work {
	return <-w.queue
}
func TypeConvertErr(from, to interface{}) error {
	return fmt.Errorf("could not convert %#v, (%T) to %T", from, from, to)
}
