package commander

import (
	"context"
	"fmt"
	"strings"
)

// a Map of actions that do work in order, mutating Commands.conf
// actions can be overridden by calling Commands.Set, and performed concurrently
// by executing the function returned from Commands.Get
// default actions are provided, though they, as well, can be overridden
type Commands struct {
	opts     []opt
	cmds     map[string]Action
	workChan *workChan
	conf     *Config
	last     *Action
}

func NewCommands(c *Config, opts ...opt) *Commands {
	cmds := (&Commands{}).New(c, opts)
	return cmds
}

func (c *Commands) New(conf *Config, opts []opt) *Commands {
	c.conf = conf
	c.opts = opts
	c.cmds = make(map[string]Action)
	c.workChan = newWorkChan(10)
	c.Set(HelpAction{cmds: c})
	c.Set(LoadAction{})
	c.Set(SaveAction{})
	c.Set(Build().WithNameV("print-config").WithExecuteVoid(func(c *Config) (interface{}, error) {
		fmt.Println(prettyJ(c))
		return *c, nil
	}).WithTagsV("default"))
	c.Set(Build().WithNameV("tags").WithVoidExecuteVoid(func(_ *Config) error {
		fmt.Printf("Known Tags:\n\t%v\n", strings.Join(c.KnownTags(), "\n\t"))
		return nil
	}).WithTagsV("default"))
	c.Set(Build().WithNameV("last").WithPayload(func(_ *Config) (interface{}, error) {
		if c.last == nil {
			return nil, Skip
		}
		return c.workChan.CachedResults[(*c.last).Name()], nil
	}).WithExecute(func(_ *Config, p interface{}) (interface{}, error) {
		fmt.Printf("\n%s\n", PrettyJson(p))
		return p, nil
	}).WithTagsV("default"))
	c.Set(Build().WithNameV("quit").WithPayloadV(nil, Quit).WithTagsV("default"))
	c.Set(Build().WithNameV("lookup").WithTagsV("default").
		WithPayload(func(*Config) (interface{}, error) {
			var alias string
			if err := scan("lookup last result to which command?", &alias); err != nil {
				return nil, err
			}
			return alias, nil
		}).
		WithExecute(func(_ *Config, payload interface{}) (interface{}, error) {
			name, ok := payload.(string)
			if !ok {
				return nil, fmt.Errorf("payload was not string %#v", name)
			}

			fmt.Printf("result to %s:\n %v\n", name, prettyJ(c.workChan.CachedResults[name]))

			return c.workChan.CachedResults[name], nil
		}))
	c.Set(Build().WithNameV("filter").WithTagsV("default").
		WithPayload(func(*Config) (interface{}, error) {
			tags := ""
			if err := scan("enter tags to filter by separated by space:", &tags); err != nil {
				return nil, err
			}
			ts := strings.Split(tags, " \t")
			return ts, nil
		}).
		WithExecute(func(_ *Config, payload interface{}) (interface{}, error) {
			ts, ok := payload.([]string)
			if !ok {
				return nil, fmt.Errorf("payload was not []string")
			}

			msg := fmt.Sprintf("actions in tag group:\n %+v \n", ts)

			for _, v := range c.FilterActions(ts...) {
				msg += "\t" + v.Name() + "\n"
			}
			fmt.Print(msg)

			return ts, nil
		}))
	c.Set(Build().WithNameV("aliases").WithTagsV("default").
		WithPayload(func(*Config) (interface{}, error) {
			var alias string
			return alias, scan("alias to which command?", &alias)
		}).
		WithExecute(func(_ *Config, payload interface{}) (interface{}, error) {
			ts, ok := payload.(string)
			if !ok {
				return nil, fmt.Errorf("payload was not string")
			}

			msg := fmt.Sprint("commands aliased to %s:\n", ts)

			for _, v := range c.Aliases(ts) {
				msg += "\t" + v + "\n"
			}
			fmt.Print(msg)

			return ts, nil
		}))

	go c.workChan.Start(conf)

	return c
}

func (c *Commands) Set(a Action, additionalKeys ...string) {
	for _, v := range additionalKeys {
		c.cmds[v] = a
	}
	c.cmds[a.Name()] = a
}

func (c *Commands) Wrap(a Action) func() (*Work, error) {
	return c.processor(a)
}

func (c *Commands) Remove(keys ...string) {
	for _, v := range keys {
		c.cmds[v] = nil
	}
}
func (c *Commands) Get(key string) func() (*Work, error) {
	if k, ok := c.cmds[strings.TrimSpace(strings.ToLower(key))]; k != nil && ok {
		return c.processor(k)
	}
	fmt.Println("\tunknown command: " + key)
	return c.processor(c.cmds["help"])
}

func (c *Commands) LatestResult(a Action) *Work {
	return c.workChan.CachedResults[a.Name()]
}

func (c *Commands) KnownCommands() (out []string) {
	for k, v := range c.cmds {
		if v != nil {
			out = append(out, k)
		}
	}
	return
}
func (c *Commands) FilterActions(tags ...string) (out []Action) {
	ts := make(map[string]bool)
	for _, v := range tags {
		ts[v] = true
	}
	atLeastOneMatch := func(others []string) bool {
		for _, v := range others {
			if ts[v] {
				return true
			}
		}
		return false
	}
	for _, v := range c.cmds {
		if atLeastOneMatch(v.Tags()) {
			out = append(out, v)
		}
	}
	return
}
func (c *Commands) Aliases(name string) (out []string) {
	for k, v := range c.cmds {
		if v.Name() == c.cmds[name].Name() {
			out = append(out, k)
		}
	}
	return
}
func (c *Commands) KnownTags() (out []string) {
	o := make(map[string]bool)
	for _, i := range c.cmds {
		for _, j := range i.Tags() {
			o[j] = true
		}
	}
	for k := range o {
		out = append(out, k)
	}
	return
}

func (c *Commands) processor(a Action) func() (*Work, error) {
	setLast := func() bool {
		for _, v := range a.Tags() {
			if v == "default" {
				return false
			}
		}
		return true
	}
	// the exact same as A, but with a No-op execute func
	skipA := Override(a).WithExecute(NopParts().Execute())
	return func() (*Work, error) {
		dashes(fmt.Sprintf("executing action %s", a.Name()))

		payload, err := a.Payload(c.conf)
		var work *Work
		if _, ok := err.(SkipExecute); ok {
			fmt.Println("skipping execution function")
			work = workFromAction(skipA, payload)
			close(work.wait)
		} else if err != nil {
			return nil, err
		} else {
			work = workFromAction(a, payload)
			c.workChan.Queue(work)
		}
		if setLast() {
			c.last = &a
		}
		go func() {
			if err := work.Wait(context.Background()); err == nil {
				for k, v := range a.Additions(c.conf) {
					c.Set(v, k)
				}
				c.Remove(a.Removals()...)
			}
			dashes(fmt.Sprintf("finished action %s\n, examine it with lookup result", a.Name()))

		}()
		return work, nil
	}
}
