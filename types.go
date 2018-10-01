package commander

import (
	"context"
	"fmt"
	"time"
)

// opt is a key and value pair where the value is overridable
type opt struct {
	key   string
	value string
}

func OptZero(key string) opt         { return opt{key: key} }
func Opt(key, value string) opt      { return opt{key, value} }
func (o opt) Set(v string) opt       { return opt{key: o.key, value: v} }
func (o opt) Read() (string, string) { return o.key, o.value }

// the error returned whenever Commands.Get("quit")() is called
var Quit = QuitError{}

type QuitError struct{}

func (QuitError) Error() string {
	return "quit"
}

// the error to return from a payload function if you would like
// to skip the execution function of the action
var Skip = SkipExecute{}

type SkipExecute struct{}

func (SkipExecute) Error() string {
	return "skip"
}

// Could be anything, but helps distinguish between payload
// which is less significant
type Config interface{}

type actionParts struct {
	name      Name
	desc      Desc
	payload   Payload
	execute   Execute
	additions Additions
	removals  Removals
	tags      Tags
}

func (a actionParts) Name() Name           { return a.name }
func (a actionParts) Desc() Desc           { return a.desc }
func (a actionParts) Payload() Payload     { return a.payload }
func (a actionParts) Execute() Execute     { return a.execute }
func (a actionParts) Additions() Additions { return a.additions }
func (a actionParts) Removals() Removals   { return a.removals }
func (a actionParts) Tags() Tags           { return a.tags }
func (a actionParts) Action() Action {
	return Build().
		WithName(a.name).
		WithDesc(a.desc).
		WithPayload(a.payload).
		WithExecute(a.execute).
		WithAdditions(a.additions).
		WithRemovals(a.removals).
		WithTags(a.tags)
}

// NopParts returns a new actionParts struct without having to provide an action
func NopParts() actionParts {
	return Break(NopAction{})
}

// Break returns a struct that can access the individual Action functions as
// Generic Versions  of the interface{} they are each supposed to implement
func Break(action Action) actionParts {
	return actionParts{
		name:      func() string { return action.Name() },
		desc:      func() string { return action.Desc() },
		tags:      func() []string { return action.Tags() },
		removals:  func() []string { return action.Removals() },
		additions: func(c *Config) map[string]Action { return action.Additions(c) },
		payload:   func(c *Config) (interface{}, error) { return action.Payload(c) },
		execute:   func(c *Config, payload interface{}) (interface{}, error) { return action.Execute(c, payload) },
	}
}

// The function signiture of Action.Name function
type Name func() string

func (n Name) From(s string) Name    { return func() string { return s } }
func (n Name) Prefix(o Name) Name    { return func() string { return o() + n() } }
func (n Name) PrefixV(o string) Name { return func() string { return o + n() } }
func (n Name) Suffix(o Name) Name    { return func() string { return n() + o() } }
func (n Name) SuffixV(o string) Name { return func() string { return n() + o } }

// The function signiture of Action.Desc function
type Desc func() string

func (n Desc) From(s string) Desc { return func() string { return s } }

// The function signiture of the Action.Execute function
type Execute func(*Config, interface{}) (interface{}, error)

// FromE's input matches the output of a Execute function.
// So this creates an Execute function out of the output of a ran Execute function
func (e Execute) FromE(f interface{}, err error) Execute {
	return func(*Config, interface{}) (interface{}, error) { return f, err }
}

// From is a shortcut for `Execute{}.FromE(f, nil)`
// where <f> is the result of some Execute function
func (e Execute) From(f interface{}) Execute { return e.FromE(f, nil) }

// Chain is used to merge together Execute functions.
// var ein interface{}
// var e Execute
// var f Execute
// g := e.Chain(f)
// g(in)
// // is the same as
// if eout, err := e(ein); err == nil { f(eout) }
func (e Execute) Chain(fs ...Execute) Execute {
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
type Payload func(*Config) (interface{}, error)

func (p Payload) FromE(q interface{}, err error) Payload {
	return func(*Config) (interface{}, error) { return q, err }
}
func (p Payload) From(q interface{}) Payload { return p.FromE(q, nil) }

// Returns a new Payload composed of p and all of the Payloads in qs (q).
// The result of the returned Payload will always be []interface{}.
// If the results of p or q are already []inteface{},
// the results are concated. If the results are not slices, they are appended
func (p Payload) ChainSlice(qs ...Payload) Payload {
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

// Returns a new Payload composed of p and all of the Payloads in qs (q).
// The result of the returned Payload will always be map[string]interface{}.
// If the results of p or q are already map[string]inteface{},
// the results are combined, where q overrides p's results if duplicate keys are present.
// If the result of p is not a map[string]inteface{}, the result will occupy the key: "" in the return map
// If the result of q is not a map[string]interface{}, the result will occupy the key: "0","1","..." in the return map
func (p Payload) ChainMap(qs ...Payload) Payload {
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
type Additions func(*Config) map[string]Action

func (a Additions) From(b map[string]Action) Additions {
	return func(*Config) map[string]Action { return b }
}

// the function signiture of the Action.Removals function
type Removals func() []string

func (r Removals) From(s []string) Removals { return func() []string { return s } }

// Tags are the function signiture of the Action.Tags function
type Tags func() []string

// KV represents a Question that is scannable.
// KV.Q is the question that will be given to scan.
// KV.Key is the key field, or variable name, that was scanned.
// KV.Hint is the type of variable that to the type that needs to be.
// STR = string, INT = int64, FLO = float64
type KV struct {
	Q       string
	Key     string
	Hint    ScanType
	Default interface{}
}

// NewKV returns a  new KV struct. <hint> will be the type of value used.
func NewKV(q, key string, hint ScanType) KV {
	kv := KV{
		Q:    q,
		Key:  key,
		Hint: hint,
	}
	switch hint {
	case STR:
		kv.Default = ""
	case INT:
		kv.Default = int64(0)
	case FLO:
		kv.Default = float64(0)
	}
	return kv
}

// WithDefault returns this KV struct, but with a different Default value for if the call to KV.Scan fails
func (q KV) WithDefault(def interface{}) KV {
	q.Default = def
	return q
}

// Scan returns the key, value, and any error encountered during fmt.Scanln of user's input
func (q KV) Scan() (string, interface{}, error) {
	// return the result if err is nil, otherwise return the default
	handleScan := func(res interface{}, err error) (interface{}, error) {
		if err != nil {
			return q.Default, err
		}
		return res, err
	}
	var val interface{}
	var err error
	switch q.Hint {
	case STR:
		p := ""
		err := scan(q.Q, &p)
		val, err = handleScan(p, err)
	case INT:
		p := int64(0)
		err := scan(q.Q, &p)
		val, err = handleScan(p, err)
	case FLO:
		p := float64(0)
		err := scan(q.Q, &p)
		val, err = handleScan(p, err)
	}
	return q.Key, val, err
}
func (q KV) MustScan() (string, interface{}) {
	if k, v, err := q.Scan(); err != nil {
		panic("error scanning a value that must be scanned: " + err.Error())
	} else {
		return k, v
	}
}

type ScanType int

const (
	STR ScanType = iota
	INT
	FLO
)

// Adds this kv to the map parameter
func (q KV) AddTo(m map[string]KV) { m[q.Key] = q }

// returns a map of all provided the KVs with key being KV.Key and value being the KV itself
func MapKVs(kvs ...KV) map[string]KV {
	out := make(map[string]KV)
	for _, v := range kvs {
		out[v.Key] = v
	}
	return out
}

type Work struct {
	Name    string
	job     Execute
	payload interface{}
	wait    chan struct{}
	// job     func(*Config, interface{}) (interface{}, error)
	// only populated after Do is called on the result
	Success    bool
	Failure    bool
	Result     interface{}
	err        error
	FinishedAt time.Time
	CreatedAt  time.Time
}

func workFromAction(a Action, payload interface{}) *Work {
	return &Work{
		Name:      a.Name(),
		job:       a.Execute,
		payload:   payload,
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

func (w *Work) Res() (interface{}, error) {
	return w.Result, w.err
}

func (w *Work) do(conf *Config) error {
	defer func() {
		close(w.wait)
	}()

	res, err := w.job(conf, w.payload)
	w.FinishedAt = time.Now()
	if err != nil {
		w.Success = false
		w.Failure = true
		w.Result = err.Error()
		w.err = err
		return err
	} else {
		w.Success = true
		w.Failure = false
		w.Result = res
		w.err = nil
		return nil
	}
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
