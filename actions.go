package commander

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"
)

type Action interface {
	Name
	// use questions to setup a payload and return it, since questions can only be run on the main thread
	Payload(*Config) (interface{}, error)
	// the interface as an arg will be the one from the payload, NO QUESTIONS can be setup from here, it will be
	// ran async in a go routine
	Execute(*Config, interface{}) (interface{}, error)
	Additions(*Config) map[string]Action
	Removals() []string
	Tags() []string
}

type BuilderAction struct {
	name      NameFunc
	payload   PayloadFunc
	execute   ExecuteFunc
	additions AdditionsFunc
	removals  RemovalsFunc
	tags      TagsFunc
}

func (o *BuilderAction) WithName(n NameFunc) *BuilderAction {
	o.name = n
	return o
}
func (o *BuilderAction) WithPayload(p PayloadFunc) *BuilderAction {
	o.payload = p
	return o
}
func (o *BuilderAction) WithExecute(e ExecuteFunc) *BuilderAction {
	o.execute = e
	return o
}
func (o *BuilderAction) WithAdditions(a AdditionsFunc) *BuilderAction {
	o.additions = a
	return o
}
func (o *BuilderAction) WithRemovals(r RemovalsFunc) *BuilderAction {
	o.removals = r
	return o
}
func (o *BuilderAction) WithTags(t TagsFunc) *BuilderAction {
	o.tags = t
	return o
}
func (o *BuilderAction) Payload(c *Config) (interface{}, error)                { return o.payload(c) }
func (o *BuilderAction) Execute(c *Config, p interface{}) (interface{}, error) { return o.execute(c, p) }
func (o *BuilderAction) Additions(c *Config) map[string]Action                 { return o.additions(c) }
func (o *BuilderAction) Removals() []string                                    { return o.removals() }
func (o *BuilderAction) Name() string                                          { return o.name() }
func (o BuilderAction) Tags() []string                                         { return o.tags() }

func BuildOverriding(parent Action) *BuilderAction {
	return &BuilderAction{
		name:      parent.Name,
		payload:   parent.Payload,
		execute:   parent.Execute,
		additions: parent.Additions,
		removals:  parent.Removals,
		tags:      parent.Tags,
	}
}
func Build() *BuilderAction { return BuildOverriding(NopAction{}) }

type NopAction struct{}

func (NopAction) Name() string                                              { return "" }
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
func (s LoadAction) Additions(*Config) map[string]Action { return nil }
func (s LoadAction) Removals() []string                  { return nil }
func (s LoadAction) Name() string                        { return "load" }
func (s LoadAction) Tags() []string                      { return []string{"load", "default"} }

type HelpAction struct {
	cmds *Commands
}

func (s HelpAction) Payload(conf *Config) (_ interface{}, _ error) { return }
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
func (s HelpAction) Additions(*Config) map[string]Action { return nil }
func (s HelpAction) Removals() []string                  { return nil }
func (s HelpAction) Name() string                        { return "help" }
func (s HelpAction) Tags() []string                      { return []string{"default", "help"} }

type SaveAction struct{}

func (s SaveAction) Payload(c *Config) (interface{}, error) {
	var filename string

	return filename, scan("type save path, or leave empty for default. \n(%s)", &filename)
}

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
func (s SaveAction) Additions(*Config) map[string]Action { return nil }
func (s SaveAction) Removals() []string                  { return nil }
func (s SaveAction) Name() string                        { return "save" }
func (s SaveAction) Tags() []string                      { return []string{"save", "default"} }

type WrapNameAction struct {
	newName   string
	oldAction Action
}

func (s WrapNameAction) New(name string, action Action) WrapNameAction {
	s.newName = name
	s.oldAction = action

	return s
}
func (s WrapNameAction) Execute(conf *Config, payload interface{}) (interface{}, error) {
	return s.oldAction.Execute(conf, payload)
}

func (s WrapNameAction) Additions(c *Config) map[string]Action { return s.oldAction.Additions(c) }
func (s WrapNameAction) Removals() []string                    { return s.oldAction.Removals() }
func (s WrapNameAction) Name() string                          { return s.newName }
func (s WrapNameAction) Tags() []string                        { return append(s.Tags(), s.newName) }

func NewNoPayloadAction(name string, exec func(*Config) (interface{}, error)) NoPayloadAction {
	return NoPayloadAction{}.New(name, exec)
}

type NoPayloadAction struct {
	exec func(*Config) (interface{}, error)
	name string
}

func (s NoPayloadAction) New(name string, exec func(*Config) (interface{}, error)) NoPayloadAction {
	s = NoPayloadAction{exec, name}
	return s
}
func (s NoPayloadAction) Payload(*Config) (_ interface{}, _ error) { return }
func (s NoPayloadAction) Execute(conf *Config, _ interface{}) (interface{}, error) {
	return s.exec(conf)
}
func (s NoPayloadAction) Additions(*Config) map[string]Action { return nil }
func (s NoPayloadAction) Removals() []string                  { return nil }
func (s NoPayloadAction) Name() string                        { return s.name }
func (s NoPayloadAction) Tags() []string                      { return []string{"none"} }

type WatchAction struct {
	cmds   Commands
	name   string
	action Action
	ticker *time.Ticker
}

func (s WatchAction) New(name string, action Action, cmds Commands) WatchAction {
	s.name = name
	s.action = action
	s.ticker = time.NewTicker(10 * time.Second)
	s.cmds = cmds

	return s
}

func (w *WatchAction) Payload(conf *Config) (interface{}, error) {
	return w.action.Payload(conf)
}

func (w *WatchAction) Execute(conf *Config, payload interface{}) (interface{}, error) {
	go func() {
		t := <-w.ticker.C
		fmt.Printf("triggering watch: %s at: %s\n", w.name, t.Format(time.Kitchen))
		w.action.Execute(conf, payload)
	}()
	return nil, nil
}
func (w *WatchAction) Additions(*Config) map[string]Action {
	return map[string]Action{
		"stop-" + w.name: NoPayloadAction{}.New("stop-"+w.name, func(c *Config) (interface{}, error) {
			w.ticker.Stop()
			return nil, nil
		}),
	}
}
func (w *WatchAction) Removals() []string { return w.action.Removals() }
func (w *WatchAction) Name() string       { return w.name }
func (w *WatchAction) Tags() []string     { return []string{"watch", "repeating", w.name} }

func MakeTrigger(parent Action, children ...Action) Action {
	m := make(map[string]Action)
	for _, v := range children {
		m[v.Name()] = v
	}
	return BuildOverriding(parent).WithAdditions(func(*Config) map[string]Action { return m })
}

func PrintAction(name, msg string) Action {
	return NoPayloadAction{}.New(name, func(*Config) (_ interface{}, _ error) {
		fmt.Println(msg)
		return
	})
}
