package commander

import (
	"fmt"
	"strings"
	"time"
)

type Commands struct {
	cmds     map[string]func() error
	workChan *WorkChan
}

func NewCommands(c *Config, actions ...Action) *Commands {
	cmds := Commands{}.new(c)
	for _, a := range actions {
		cmds.Set(a, c)
	}
	return cmds
}

func (c Commands) new(conf *Config) *Commands {
	c.Set(HelpAction{cmds: &c}, conf)
	c.Set(LoadAction{}, conf)
	c.Set(SaveAction{}, conf)
	c.Set(NoPayloadAction{}.New("print-config", func(c *Config) (interface{}, error) {
		fmt.Println(prettyJ(c))
		return *c, nil
	}), conf, "config")
	c.Set(NewNoPayloadAction("lookup", func(*Config) (interface{}, error) {
		name := ""
		if err := scan("name of result to lookup?", &name); err != nil {
			return nil, err
		}
		fmt.Printf("\n%v\n", prettyJ(c.workChan.CachedResults[name]))

		return nil, nil
	}), conf)

	return &c
}

func (c *Commands) Set(a Action, conf *Config, additionalKeys ...string) {
	for _, v := range additionalKeys {
		c.cmds[v] = c.processor(a, conf)
	}
	c.cmds[a.Name()] = c.processor(a, conf)
}

func (c *Commands) Wrap(a Action, conf *Config) func() error {
	return c.processor(a, conf)
}

func (c *Commands) Remove(keys ...string) {
	for _, v := range keys {
		c.cmds[v] = nil
	}
}
func (c *Commands) Get(key string) func() error {
	if k, _ := c.cmds[strings.TrimSpace(strings.ToLower(key))]; k != nil {
		return k
	}
	fmt.Println("\tunknown command: " + key)
	return c.cmds["help"]
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

func (c *Commands) processor(a Action, conf *Config) func() error {
	return func() error {
		dashes(fmt.Sprintf("executing action %s", a.Name()))
		payload, err := a.Payload(conf)
		if err != nil {
			return err
		}

		c.workChan.Queue(WorkFromAction(a, payload))

		t := time.NewTicker(time.Millisecond * 500)
		go func() {
			for {
				<-t.C
				item, ok := c.workChan.CachedResults[a.Name()]
				var zero time.Time
				if !ok || item == nil || item.FinishedAt != zero {
					break
				}
			}
			t.Stop()
			for k, v := range a.Additions(conf) {
				c.Set(v, conf, k)
			}
			c.Remove(a.Removals()...)

			dashes(fmt.Sprintf("finished action %s\n, examine it with lookup result", a.Name()))
		}()
		return nil
	}
}
