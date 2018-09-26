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
	Name
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
	name      NameFunc
	desc      DescFunc
	payload   PayloadFunc
	execute   ExecuteFunc
	additions AdditionsFunc
	removals  RemovalsFunc
	tags      TagsFunc
}

func (o *builderAction) WithNameFunc(n NameFunc) *builderAction {
	o.name = n
	return o
}
func (o *builderAction) WithDescFunc(d DescFunc) *builderAction {
	o.desc = d
	return o
}
func (o *builderAction) WithPayloadFunc(p PayloadFunc) *builderAction {
	o.payload = p
	return o
}
func (o *builderAction) WithExecuteFunc(e ExecuteFunc) *builderAction {
	o.execute = e
	return o
}
func (o *builderAction) WithAdditionsFunc(a AdditionsFunc) *builderAction {
	o.additions = a
	return o
}
func (o *builderAction) WithRemovalsFunc(r RemovalsFunc) *builderAction {
	o.removals = r
	return o
}
func (o *builderAction) WithTagsFunc(t TagsFunc) *builderAction {
	o.tags = t
	return o
}
func (o *builderAction) WithName(n string) *builderAction {
	o.name = func() string { return n }
	return o
}
func (o *builderAction) WithDesc(d string) *builderAction {
	o.desc = func() string { return d }
	return o
}
func (o *builderAction) WithPayload(p interface{}, err error) *builderAction {
	o.payload = func(*Config) (interface{}, error) { return p, err }
	return o
}
func (o *builderAction) WithForkPayloadsFunc(p map[string]PayloadFunc) *builderAction {
	o.payload = ForkPayloads(p)
	return o
}
func (o *builderAction) WithForkPayloads(p map[string]interface{}) *builderAction {
	newMap := make(map[string]PayloadFunc)
	for k, v := range p {
		newMap[k] = func(*Config) (interface{}, error) {
			return v, nil
		}
	}
	o.payload = ForkPayloads(newMap)
	return o
}
func (o *builderAction) WithAggregatePayloadFuncs(p map[string]PayloadFunc) *builderAction {
	o.payload = CombinePayloads(p)
	return o
}
func (o *builderAction) WithExecute(result interface{}, err error) *builderAction {
	o.execute = func(*Config, interface{}) (interface{}, error) { return result, err }
	return o
}
func (o *builderAction) WithExecuteOfMapPayload(pFunc func(*Config, map[string]interface{}) (interface{}, error)) *builderAction {
	o.execute = func(c *Config, p interface{}) (interface{}, error) {
		converted, ok := p.(map[string]interface{})
		if !ok {
			return nil, TypeConvertErr(p, map[string]interface{}{})
		}
		return pFunc(c, converted)
	}
	return o
}
func (o *builderAction) WithExecuteOfStringPayload(pFunc func(*Config, string) (interface{}, error)) *builderAction {
	o.execute = func(c *Config, p interface{}) (interface{}, error) {
		converted, ok := p.(string)
		if !ok {
			return nil, TypeConvertErr(p, "")
		}
		return pFunc(c, converted)
	}
	return o
}
func (o *builderAction) WithExecuteOfInt64Payload(pFunc func(*Config, int64) (interface{}, error)) *builderAction {
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
func (o *builderAction) WithExecuteOfNoPayload(pFunc func(*Config) (interface{}, error)) *builderAction {
	o.execute = func(c *Config, _ interface{}) (interface{}, error) {
		return pFunc(c)
	}
	return o
}
func (o *builderAction) WithAdditions(a map[string]Action) *builderAction {
	o.additions = func(*Config) map[string]Action { return a }
	return o
}
func (o *builderAction) WithRemovals(r []string) *builderAction {
	o.removals = func() []string { return r }
	return o
}
func (o *builderAction) WithRemovalsV(rs ...string) *builderAction { return o.WithRemovals(rs) }
func (o *builderAction) WithTags(t []string) *builderAction {
	o.tags = func() []string { return t }
	return o
}
func (o *builderAction) WithTagsV(ts ...string) *builderAction { return o.WithTags(ts) }

func (o *builderAction) Payload(c *Config) (interface{}, error)                { return o.payload(c) }
func (o *builderAction) Execute(c *Config, p interface{}) (interface{}, error) { return o.execute(c, p) }
func (o *builderAction) Additions(c *Config) map[string]Action                 { return o.additions(c) }
func (o *builderAction) Removals() []string                                    { return o.removals() }
func (o *builderAction) Name() string                                          { return o.name() }
func (o *builderAction) Desc() string                                          { return o.desc() }
func (o builderAction) Tags() []string                                         { return o.tags() }

// BETTER DOCUMENTATION COMING
func BuildOverriding(parent Action) *builderAction {
	return &builderAction{
		name:      parent.Name,
		payload:   parent.Payload,
		execute:   parent.Execute,
		additions: parent.Additions,
		removals:  parent.Removals,
		tags:      parent.Tags,
	}
}

// BETTER DOCUMENTATION COMING
func Build() *builderAction { return BuildOverriding(NopAction{}) }

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
func (LoadAction) Tags() []string                      { return []string{"load", "default"} }

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
func (HelpAction) Tags() []string                      { return []string{"default", "help"} }

type SaveAction struct{}

func (s SaveAction) Payload(c *Config) (interface{}, error) {
	var filename string

	return filename, scan("type save path, or leave empty for default. \n(%s)", &filename)
}

// must Always have a string payload that is the filepath to save
func (s SaveAction) Execute(c *Config, payload interface{}) (interface{}, error) {
	ans, ok := payload.(string)
	if !ok {
		return nil, fmt.Errorf("payload was not a string: %v", payload)
	}
	ReplaceDotSlash(&ans)
	ReplaceHome(&ans)
	// TODO this works?
	conf, ok := (interface{})(c).(struct{ File string })
	if !ok {
		return nil, fmt.Errorf("save action expects a `File` field on your config")
	}
	conf.File = ans
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
func (SaveAction) Tags() []string                      { return []string{"save", "default"} }

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
		"stop-" + w.action.Name(): Build().WithExecuteOfNoPayload(func(c *Config) (interface{}, error) {
			w.ticker.Stop()
			return nil, nil
		}),
	}
}
func (w *WatchAction) Removals() []string { return w.action.Removals() }
func (w *WatchAction) Name() string       { return w.action.Name() }
func (w *WatchAction) Tags() []string     { return []string{"watch", "repeating", w.action.Name()} }

func MakeTrigger(parent Action, children ...Action) Action {
	m := make(map[string]Action)
	for _, v := range children {
		m[v.Name()] = v
	}
	return BuildOverriding(parent).WithAdditions(m)
}

func PrintAction(name, msg string) Action {
	return Build().WithName(name).WithExecuteOfNoPayload(func(*Config) (_ interface{}, _ error) {
		fmt.Println(msg)
		return
	})
}

// return a Payload function that asks the user to pick between the keys in the map
// it performs the named PayloadFunction if it exists, and returns its result as the payload
// unknown keys result in an error
// TODO
func ForkPayloads(payloads map[string]PayloadFunc) PayloadFunc {
	return func(c *Config) (interface{}, error) {
		temp := ""
		keys := make([]string, 0)
		for k, _ := range payloads {
			keys = append(keys, k)

		}
		var payloadF PayloadFunc
		retry(3, func() error {
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
		return payloadF(c)
	}
}

// returns a payload function that always performs all the input payload functions,
// and returns map[string]inteface{} as its payload
// This can then be relied on in the ExecuteFunctions to always be castable to map[string]inteface{}
func CombinePayloads(payloads map[string]PayloadFunc) PayloadFunc {
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
