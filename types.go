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
func (e ExecuteFunc) Chain(fs ...ExecuteFunc) ExecuteFunc {
	return func(conf *Config, payload interface{}) (interface{}, error) {
		res, err := e(conf, payload)
		if err != nil {
			return nil, err
		}
		for _, f := range fs {
			res, err = f(conf, res)
			if err != nil {
				return nil, err
			}
		}
		return res, nil
	}
}

// the function signiture of the Action.Payload function
type PayloadFunc func(*Config) (interface{}, error)

// Returns a new PayloadFunc composed of p and all of the PayloadFuncs in qs (q).
// The result of the returned PayloadFunc will always be []interface{}.
// If the results of p or q are already []inteface{},
// the results are concated. If the results are not slices, they are appended
func (p PayloadFunc) ChainSlice(qs ...PayloadFunc) PayloadFunc {
	return func(conf *Config) (interface{}, error) {
		var res []interface{}
		pres, err := p(conf)
		if err != nil {
			return nil, err
		}
		if r, ok := pres.([]interface{}); ok {
			res = r
		} else {
			res = append(res, pres)
		}
		for _, q := range qs {
			qres, err := q(conf)
			if err != nil {
				return nil, err
			}
			if r, ok := qres.([]interface{}); ok {
				res = append(res, r...)
			} else {
				res = append(res, qres)
			}
		}
		return res, nil
	}
}

// Returns a new PayloadFunc composed of p and all of the PayloadFuncs in qs (q).
// The result of the returned PayloadFunc will always be map[string]interface{}.
// If the results of p or q are already map[string]inteface{},
// the results are combined, where q overrides p's results if duplicate keys are present.
// If the result of p is not a map[string]inteface{}, the result will occupy the key: "" in the return map
// If the result of q is not a map[string]interface{}, the result will occupy the key: "0","1","..." in the return map
func (p PayloadFunc) ChainMap(qs ...PayloadFunc) PayloadFunc {
	return func(conf *Config) (interface{}, error) {
		var res map[string]interface{}
		pres, err := p(conf)
		if err != nil {
			return nil, err
		}
		if r, ok := pres.(map[string]interface{}); ok {
			res = r
		} else {
			res = map[string]interface{}{"": pres}
		}
		for i, q := range qs {
			qres, err := q(conf)
			if err != nil {
				return nil, err
			}
			if r, ok := qres.(map[string]interface{}); ok {
				for k, v := range r {
					res[k] = v
				}
			} else {
				res[string(i)] = qres
			}
		}
		return res, nil

	}
}

// the function signiture of the Action.Addtions function
type AdditionsFunc func(*Config) map[string]Action

// the function signiture of the Action.Removals function
type RemovalsFunc func() []string

// the function signiture of the Action.Tags function
type TagsFunc func() []string

type Name interface {
	Name() string
}

// represents a Question that is scannable
// KV.Q is the question that will be given to scan
// KV.Key is the key field, or variable name, that was scanned
// KV.Hint is variable that matches the type that needs to be scanned into
type KV struct {
	Q    string
	Key  string
	Hint interface{}
}

func NewKV(q, key string, hint interface{}) KV {
	if hint == nil {
		hint = ""
	}
	return KV{
		Q:    q,
		Key:  key,
		Hint: hint,
	}
}

func (q KV) Scan() (string, interface{}, error) {
	return q.Key, q.Hint, scan(q.Q, &q.Hint)
}
func (q KV) MustScan() (string, interface{}) {
	if k, v, err := q.Scan(); err != nil {
		panic("error scanning a value that must be scanned: " + err.Error())
	} else {
		return k, v
	}
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
