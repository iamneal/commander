package commander

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
)

// Actions are a helper class for Commands
// They can be created, but the Payload and Execute functions are meant
// to be used by the Command that holds the Action.
// Actions are performed in two stages
// on the main thread with the Comamnds, payload is called with the
// Config given to the Commands.
// though not enforeced by the api (yet) Conifg should be treated as read only at this point
// execute is performed nonblocking, and the result is returned which can then be waited on
type Action interface {
	Name() string
	Desc() string
	Payload(*Config) (interface{}, error)
	// the interface as an arg will be the one from the payload, NO QUESTIONS can be setup from here, it will be
	// ran async in a go routine
	Execute(*Config, interface{}) (interface{}, error)
	Additions(*Config) map[string]Action
	Removals() []string
	Tags() []string
}

type builderAction struct {
	name      Name
	desc      Desc
	payload   Payload
	execute   Execute
	additions Additions
	removals  Removals
	tags      Tags
}

// Override takes a parent Action as input, and returns an action builder
// Use With* methods on the action builder to override Different portions of the
// parent action.
// The builderAction satisfies the Action interface, so it can, itself, be given
// to Override().
func Override(parent Action) *builderAction {
	return &builderAction{
		name:      parent.Name,
		payload:   parent.Payload,
		execute:   parent.Execute,
		additions: parent.Additions,
		removals:  parent.Removals,
		tags:      parent.Tags,
	}
}

// Build is a shortcut for calling Override(NopAction{}).  NopAction being an empty action
// that does nothing, this is the starting point for building Actions from scratch with
// The builderAction.
func Build() *builderAction { return Override(NopAction{}) }

// WithName will return the result of "n" when the action's Name() function is called.
// it returns itself for chaining.
func (o *builderAction) WithName(n Name) *builderAction {
	o.name = n
	return o
}

// WithDesc will return the result of "d" when the action's Desc() function is called.
// it returns itself for chaining.
func (o *builderAction) WithDesc(d Desc) *builderAction {
	o.desc = d
	return o
}

// WithPayload will return the result of "p" when the action's Payload() function is called.
// it returns itself for chaining.
func (o *builderAction) WithPayload(p Payload) *builderAction {
	o.payload = p
	return o
}

// WithExecute will return the result of "e" when the action's Execute() function is called.
// it returns itself for chaining.
func (o *builderAction) WithExecute(e Execute) *builderAction {
	o.execute = e
	return o
}

// WithAdditions will return the result of "a" when the action's Additions() function is called.
// it returns itself for chaining.
func (o *builderAction) WithAdditions(a Additions) *builderAction {
	o.additions = a
	return o
}

// WithRemovals will return the result of "r" when the action's Removal() function is called.
// it returns itself for chaining.
func (o *builderAction) WithRemovals(r Removals) *builderAction {
	o.removals = r
	return o
}

// WithTags will return the result of "t" when the action's Tags() function is called.
// it returns itself for chaining.
func (o *builderAction) WithTags(t Tags) *builderAction {
	o.tags = t
	return o
}

// WithNameV creates a new Name func that returns n when the actions Name() method is called.
// it returns itself for chaining.
func (o *builderAction) WithNameV(n string) *builderAction {
	o.name = func() string { return n }
	return o
}
func (o *builderAction) WithDescV(d string) *builderAction {
	o.desc = func() string { return d }
	return o
}

// WithPayload creates a new Payload func that returns "p" and "err" when
// the builderAction's Payload method is called.
// it returns itself for chaining.
func (o *builderAction) WithPayloadV(p interface{}, err error) *builderAction {
	o.payload = func(*Config) (interface{}, error) { return p, err }
	return o
}

// WithForkPayloads creates a new Payload func by calling ForkPayloads with "p"
// it returns itself for chaining.
func (o *builderAction) WithForkPayloads(p map[string]Payload) *builderAction {
	o.payload = ForkPayloads(p)
	return o
}

// WithForkPayloadsV creates new Payload funcs that return the value stored at each key in "p".
// The builderAction's Payload function is the result of calling ForkPayloads() on this
// map of new functions.
// it returns itself for chaining.
func (o *builderAction) WithForkPayloadsV(p map[string]interface{}) *builderAction {
	newMap := make(map[string]Payload)
	for k, v := range p {
		newMap[k] = func(*Config) (interface{}, error) {
			return v, nil
		}
	}
	o.payload = ForkPayloads(newMap)
	return o
}
func (o *builderAction) WithQuestionsPayload(kvs ...KV) *builderAction {
	o.payload = func(*Config) (interface{}, error) {
		res := make(map[string]interface{})
		for _, kv := range kvs {
			key, value, err := kv.Scan()
			if err != nil {
				return nil, err
			}
			res[key] = value
		}
		return res, nil
	}
	return o
}

// WithAggregatePayload makes a new Payload func by calling CombinePayloads() on "p".
// The builderAction's Payload function is replaced by this new Payload function.
// it returns itself for chaining.
func (o *builderAction) WithAggregatePayload(p map[string]Payload) *builderAction {
	o.payload = CombinePayloads(p)
	return o
}
func (o *builderAction) WithExecuteV(result interface{}, err error) *builderAction {
	o.execute = func(*Config, interface{}) (interface{}, error) { return result, err }
	return o
}
func (o *builderAction) WithExecuteMap(p func(*Config, map[string]interface{}) (interface{}, error)) *builderAction {
	o.execute = func(c *Config, q interface{}) (interface{}, error) {
		converted, ok := q.(map[string]interface{})
		if !ok {
			return nil, TypeConvertErr(q, map[string]interface{}{})
		}
		return p(c, converted)
	}
	return o
}
func (o *builderAction) WithExecuteSlice(p func(*Config, []interface{}) (interface{}, error)) *builderAction {
	o.execute = func(c *Config, q interface{}) (interface{}, error) {
		converted, ok := q.([]interface{})
		if !ok {
			return nil, TypeConvertErr(q, []interface{}{})
		}
		return p(c, converted)
	}
	return o
}
func (o *builderAction) WithExecuteString(pFunc func(*Config, string) (interface{}, error)) *builderAction {
	o.execute = func(c *Config, p interface{}) (interface{}, error) {
		converted, ok := p.(string)
		if !ok {
			return nil, TypeConvertErr(p, "")
		}
		return pFunc(c, converted)
	}
	return o
}
func (o *builderAction) WithExecuteInt64(pFunc func(*Config, int64) (interface{}, error)) *builderAction {
	o.execute = func(c *Config, p interface{}) (interface{}, error) {
		converted, ok := p.(int64)
		if !ok {
			err := TypeConvertErr(p, "")
			convString, ok := p.(string)
			if !ok {
				return nil, err
			}
			converted, err = strconv.ParseInt(convString, 10, 64)
			if err != nil {
				return nil, err
			}
		}
		return pFunc(c, converted)
	}
	return o
}
func (o *builderAction) WithExecuteVoid(pFunc func(*Config) (interface{}, error)) *builderAction {
	o.execute = func(c *Config, _ interface{}) (interface{}, error) {
		return pFunc(c)
	}
	return o
}

// Void Execute functions
func (o *builderAction) WithVoidExecuteMap(p func(*Config, map[string]interface{}) error) *builderAction {
	o.execute = func(c *Config, q interface{}) (interface{}, error) {
		converted, ok := q.(map[string]interface{})
		if !ok {
			return nil, TypeConvertErr(q, map[string]interface{}{})
		}
		return nil, p(c, converted)
	}
	return o
}
func (o *builderAction) WithVoidExecuteSlice(p func(*Config, []interface{}) error) *builderAction {
	o.execute = func(c *Config, q interface{}) (interface{}, error) {
		converted, ok := q.([]interface{})
		if !ok {
			return nil, TypeConvertErr(q, []interface{}{})
		}
		return nil, p(c, converted)
	}
	return o
}
func (o *builderAction) WithVoidExecuteString(pFunc func(*Config, string) error) *builderAction {
	o.execute = func(c *Config, p interface{}) (interface{}, error) {
		converted, ok := p.(string)
		if !ok {
			return nil, TypeConvertErr(p, "")
		}
		return nil, pFunc(c, converted)
	}
	return o
}
func (o *builderAction) WithVoidExecuteInt64(pFunc func(*Config, int64) error) *builderAction {
	o.execute = func(c *Config, p interface{}) (interface{}, error) {
		converted, ok := p.(int64)
		if !ok {
			err := TypeConvertErr(p, "")
			convString, ok := p.(string)
			if !ok {
				return nil, err
			}
			converted, err = strconv.ParseInt(convString, 10, 64)
			if err != nil {
				return nil, err
			}
		}
		return nil, pFunc(c, converted)
	}
	return o
}
func (o *builderAction) WithVoidExecuteVoid(pFunc func(*Config) error) *builderAction {
	o.execute = func(c *Config, _ interface{}) (interface{}, error) {
		return nil, pFunc(c)
	}
	return o
}

func (o *builderAction) WithAdditionsV(a map[string]Action) *builderAction {
	o.additions = func(*Config) map[string]Action { return a }
	return o
}
func (o *builderAction) WithRemovalsV(rs ...string) *builderAction {
	o.removals = func() []string { return rs }
	return o
}

func (o *builderAction) WithTagsV(ts ...string) *builderAction {
	o.tags = func() []string { return ts }
	return o
}

// Break this action into a group of its parts
func (o *builderAction) Break() actionParts { return Break(o) }

func (o *builderAction) Payload(c *Config) (interface{}, error)                { return o.payload(c) }
func (o *builderAction) Execute(c *Config, p interface{}) (interface{}, error) { return o.execute(c, p) }
func (o *builderAction) Additions(c *Config) map[string]Action                 { return o.additions(c) }
func (o *builderAction) Removals() []string                                    { return o.removals() }
func (o *builderAction) Name() string                                          { return o.name() }
func (o *builderAction) Desc() string                                          { return o.desc() }
func (o builderAction) Tags() []string                                         { return o.tags() }

// TODO sync these up with NopAction Better
func NewName() Name { return func() string { return "" } }
func NewDesc() Desc { return func() string { return "" } }
func NewPayload() Payload {
	return func(*Config) (_ interface{}, _ error) { return }
}
func NewExecute() Execute {
	return func(c *Config, p interface{}) (_ interface{}, _ error) { return }
}
func NewAdditions() Additions {
	return func(c *Config) map[string]Action { return nil }
}
func NewRemovals() Removals { return func() []string { return nil } }
func NewTags() Tags         { return func() []string { return nil } }

type NopAction struct{}

func (NopAction) Name() string                                              { return "" }
func (NopAction) Desc() string                                              { return "" }
func (NopAction) Payload(*Config) (_ interface{}, _ error)                  { return }
func (NopAction) Execute(c *Config, p interface{}) (_ interface{}, _ error) { return }
func (NopAction) Additions(c *Config) map[string]Action                     { return nil }
func (NopAction) Removals() []string                                        { return nil }
func (NopAction) Tags() []string                                            { return nil }

type LoadAction struct{}

func (s LoadAction) Payload(conf *Config) (interface{}, error) {
	var filename string
	//TODO if not already in result list

	// this filename is given to Execute as a payload
	return filename, scan("load file", &filename)
}

func (s LoadAction) Execute(conf *Config, payload interface{}) (interface{}, error) {
	var c Config
	var d []byte

	filename, ok := payload.(string)
	if !ok {
		return nil, fmt.Errorf("could not convert payload to string: %v", filename)
	}

	e := E()
	next := func(f func(*error)) {
		e.WrapAssign(f)()
	}

	ReplaceDotSlash(&filename)
	ReplaceHome(&filename)

	next(func(err *error) { d, *err = ioutil.ReadFile(filename) })
	next(func(err *error) { *err = json.Unmarshal(d, &c) })

	if e.Err() != nil {
		fmt.Printf("error was encountered loading file: \n\t%s\nerror:\n\t%v\n", filename, e.Err())
	}
	*conf = c
	return nil, e.Err()
}
func (LoadAction) Additions(*Config) map[string]Action { return nil }
func (LoadAction) Removals() []string                  { return nil }
func (LoadAction) Name() string                        { return "load" }
func (LoadAction) Desc() string                        { return "load from a file" }
func (LoadAction) Tags() []string                      { return []string{"default"} }

type HelpAction struct {
	cmds *Commands
}

func (HelpAction) Payload(conf *Config) (_ interface{}, _ error) { return }
func (s HelpAction) Execute(conf *Config, _ interface{}) (interface{}, error) {
	instructions := "\nplease type a command: \n%s"
	commands := func() (out string) {
		for _, v := range s.cmds.KnownCommands() {
			out += "\t" + v + "\n"
		}
		return
	}()
	fmt.Printf(instructions, commands)

	return nil, nil
}
func (HelpAction) Additions(*Config) map[string]Action { return nil }
func (HelpAction) Removals() []string                  { return nil }
func (HelpAction) Name() string                        { return "help" }
func (HelpAction) Desc() string                        { return "get help for stuff" }
func (HelpAction) Tags() []string                      { return []string{"default"} }

type SaveAction struct{}

func (s SaveAction) Payload(c *Config) (interface{}, error) {
	var filename string

	return filename, scan("type save path, or leave empty for default.", &filename)
}

// must Always have a string payload that is the filepath to save
func (s SaveAction) Execute(c *Config, payload interface{}) (interface{}, error) {
	ans, ok := payload.(string)
	if !ok {
		return nil, fmt.Errorf("payload was not a string: %v", payload)
	}
	ReplaceDotSlash(&ans)
	ReplaceHome(&ans)

	bytes, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return ans, ioutil.WriteFile(ans, bytes, 0644)
}
func (SaveAction) Additions(*Config) map[string]Action { return nil }
func (SaveAction) Removals() []string                  { return nil }
func (SaveAction) Name() string                        { return "save" }
func (SaveAction) Desc() string                        { return "save the config to a file" }
func (SaveAction) Tags() []string                      { return []string{"default"} }

type WrapNameAction struct {
	newName   string
	oldAction Action
}

func (s WrapNameAction) New(name string, action Action) WrapNameAction {
	s.newName = name
	s.oldAction = action

	return s
}
func (s WrapNameAction) Payload(c *Config) (interface{}, error) { return s.oldAction.Payload(c) }
func (s WrapNameAction) Execute(conf *Config, payload interface{}) (interface{}, error) {
	return s.oldAction.Execute(conf, payload)
}

func (s WrapNameAction) Additions(c *Config) map[string]Action { return s.oldAction.Additions(c) }
func (s WrapNameAction) Removals() []string                    { return s.oldAction.Removals() }
func (s WrapNameAction) Name() string                          { return s.newName }
func (s WrapNameAction) Desc() string                          { return s.oldAction.Desc() }
func (s WrapNameAction) Tags() []string                        { return append(s.Tags(), s.newName) }

// executes the child actions payload once, then
// every <tick> seconds till Commands.Get("stop-" + <name>) is called,
// Watch calls the child action's Execute function with that payload
type WatchAction struct {
	cmds   Commands
	action Action
	ticker *time.Ticker
}

func NewWatchAction(action Action, tick time.Duration, cmds Commands) WatchAction {
	return WatchAction{}.New(action, tick, cmds)
}

func (s WatchAction) New(action Action, tick time.Duration, cmds Commands) WatchAction {
	s.action = action
	s.ticker = time.NewTicker(tick)
	s.cmds = cmds

	return s
}

func (w *WatchAction) Payload(conf *Config) (interface{}, error) {
	return w.action.Payload(conf)
}

func (w *WatchAction) Execute(conf *Config, payload interface{}) (interface{}, error) {
	go func() {
		<-w.ticker.C
		w.action.Execute(conf, payload)
	}()
	return nil, nil
}
func (w *WatchAction) Additions(*Config) map[string]Action {
	return map[string]Action{
		"stop-" + w.action.Name(): Build().WithVoidExecuteVoid(func(c *Config) error {
			w.ticker.Stop()
			return nil
		}),
	}
}
func (w *WatchAction) Removals() []string { return w.action.Removals() }
func (w *WatchAction) Name() string       { return w.action.Name() }
func (w *WatchAction) Tags() []string     { return []string{"watch", "repeating", w.action.Name()} }

func PrintAction(name, msg string) Action {
	return Build().WithNameV(name).WithVoidExecuteVoid(func(*Config) error {
		fmt.Println(msg)
		return nil
	})
}

func MakeTrigger(parent Action, children ...Action) Action {
	m := make(map[string]Action)
	for _, v := range children {
		m[v.Name()] = v
	}
	return Override(parent).WithAdditionsV(m)
}

// return a Payload function that asks the user to pick between the keys in the map
// it performs the named PayloadFunction if it exists, and returns its result as the payload
// unknown keys result in an error
func ForkPayloads(payloads map[string]Payload) Payload {
	return func(c *Config) (interface{}, error) {
		temp := ""
		keys := make([]string, 0)
		for k, _ := range payloads {
			keys = append(keys, k)

		}
		var payloadF Payload
		err := retry(3, func() error {
			if err := scan("please pick between:"+strings.Join(keys, "\n\t"), &temp); err != nil {
				return err
			}
			f, ok := payloads[temp]
			if !ok || f == nil {
				return fmt.Errorf("no payload function named %s", temp)
			}
			payloadF = f
			return nil
		})
		if err != nil {
			return nil, err
		}
		return payloadF(c)
	}
}

// returns a payload function that always performs all the input payload functions,
// and returns map[string]inteface{} as its payload
// This can then be relied on in the ExecuteFunctions to always be castable to map[string]inteface{}
func CombinePayloads(payloads map[string]Payload) Payload {
	return func(c *Config) (interface{}, error) {
		resMap := make(map[string]interface{})
		for k, v := range payloads {
			if res, err := v(c); err != nil {
				resMap[k] = err
			} else {
				resMap[k] = res
			}
		}
		return resMap, nil
	}
}
