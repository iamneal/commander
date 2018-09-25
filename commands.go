package commander

import (
	"context"
	"fmt"
	"strings"
)

type Commands struct {
	cmds     map[string]Action
	workChan *WorkChan
	conf     *Config
}

func NewCommands(c *Config, actions ...Action) *Commands {
	cmds := (&Commands{}).new(c)
	for _, a := range actions {
		cmds.Set(a)
	}
	return cmds
}

func (c *Commands) new(conf *Config) *Commands {
	c.conf = conf
	c.Set(HelpAction{cmds: c})
	c.Set(LoadAction{})
	c.Set(SaveAction{})
	c.Set(NoPayloadAction{}.New("print-config", func(c *Config) (interface{}, error) {
		fmt.Println(prettyJ(c))
		return *c, nil
	}), "config")
	c.Set(PrintAction("tags", "all known tags:\n"+strings.Join(c.KnownTags(), "\n\t")))
	c.Set(Build().WithName("quit").WithPayload(nil, QuitError{}))
	c.Set(Build().WithName("lookup").
		WithPayloadFunc(func(*Config) (interface{}, error) {
			var alias string
			return alias, scan("lookup last result to which command?", &alias)
		}).
		WithExecuteFunc(func(_ *Config, payload interface{}) (interface{}, error) {
			name, ok := payload.(string)
			if !ok {
				return nil, fmt.Errorf("payload was not string %#v", name)
			}

			fmt.Printf("result to %s:\n%v\n", name, prettyJ(c.workChan.CachedResults[name]))

			return nil, nil
		}))
	c.Set(Build().WithName("filter").
		WithPayloadFunc(func(*Config) (interface{}, error) {
			tags := ""
			if err := scan("enter tags to filter by separated by space:", &tags); err != nil {
				return nil, err
			}
			ts := strings.Split(tags, " \t")
			return ts, nil
		}).
		WithExecuteFunc(func(_ *Config, payload interface{}) (interface{}, error) {
			ts, ok := payload.([]string)
			if !ok {
				return nil, fmt.Errorf("payload was not []string")
			}

			msg := fmt.Sprint("actions in tag group:\n%v\n", ts)

			for _, v := range c.FilterActions(ts...) {
				msg += "\t" + v.Name() + "\n"
			}
			fmt.Print(msg)

			return nil, nil
		}))
	c.Set(Build().WithName("aliases").
		WithPayloadFunc(func(*Config) (interface{}, error) {
			var alias string
			return alias, scan("alias to which command?", &alias)
		}).
		WithExecuteFunc(func(_ *Config, payload interface{}) (interface{}, error) {
			ts, ok := payload.(string)
			if !ok {
				return nil, fmt.Errorf("payload was not string")
			}

			msg := fmt.Sprint("commands aliased to %s:\n", ts)

			for _, v := range c.Aliases(ts) {
				msg += "\t" + v + "\n"
			}
			fmt.Print(msg)

			return nil, nil
		}))

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
	for k, _ := range o {
		out = append(out, k)
	}
	return
}

func (c *Commands) processor(a Action) func() (*Work, error) {
	return func() (*Work, error) {
		dashes(fmt.Sprintf("executing action %s", a.Name()))
		payload, err := a.Payload(c.conf)
		if err != nil {
			return nil, err
		}
		work := WorkFromAction(a, payload)
		c.workChan.Queue(work)
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
