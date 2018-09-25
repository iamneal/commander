# Commander

edit, serialize, and view data concurrently through a map of functions meant to be used 
to help build interactive termnial applications of long running processes.

## STATUS (NOT READY, ALPHA)


USEAGE:


```go
package main

import (
    "fmt"
	"github.com/iamneal/commander"
)

func main() {
    // the config could be anything.  Your actions will handle this 
    config := Something{}

    // make a new commands struct with this config and action list
    commands := NewCommands(config, commander.PrintAction("hello", "HELLO WORLD"))


    for {
        var cmd string
        fmt.Scanln("what command to run? ", &cmd)

        // command is ran here, if the command name is not found, the 
        // help command is ran commands are run in parallel, but it 
        // is safe to treat them as having sole access to the config
        // for the length of the action the result is not returned, 
        // it can be accessed through the lookup command, and the action name
        if err := commands.Get(cmd)(); err != nil {
            if _, ok: err.(commander.QuitErr); ok {
                fmt.Println("quitting...")
                break
            }
        }
    }
}
```

## TODO
- overrides action builder that sources an action as parent and overrides anything not nil through a builder
- make a saveResult and loadResult action to copy results from memory to a spot in the config, or back
- write the payload function helpers (payload func() interface, ensure payload function exists on v2 structure)
    also:
    - payload function from questions
    - payload function from pasted json
    - payload function from file
    - look into reader writer interfaces
    - payload function from scanner
- wrie a function that when given a slice of actions, gives you a commands object
- write a filter function that will filter commands and print them by tag
- write an action combiner that can take a seq of actions and gives the result of each previous action
    to the next action, and not saving the intermediate steps
- write a function that takes a grpc interface and gives you a commands object
    it will have:
    - grpc actions converters for client stream and server stream actions
