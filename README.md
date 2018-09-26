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
    commands := commander.NewCommands(config, commander.PrintAction("hello", "HELLO WORLD"))


    for {
        var cmd string
        fmt.Scanln("what command to run? ", &cmd)

        // command is ran here. If the command name is not found, the 
        // help command is ran. Commands are run in parallel, but it 
        // is safe to treat them as having sole access to the config
        // for the length of the action. 
        // the returned "Work"  will be populated when the work is done.
        if err := commands.Get(cmd)(); err != nil {
            if work, ok: err.(commander.QuitErr); ok {
                fmt.Println("quitting...")
                break
            } else {
                // Wait can be called multiple times, even if the work has been done.
                // Wait blocks till the work has been completed.
                work.Wait()
                fmt.Println(work.Result)
            }
        }
    }
}
```

## TODO
- write the payload function helpers 
    also:
    - payload function from questions
    - payload function from pasted json
    - payload function from file
    - look into reader writer interfaces
    - payload function from scanner
- write a function that takes a grpc interface and gives you a commands object
    it will have:
    - grpc actions converters for client stream and server stream actions

## ROADMAP
- update the processor internals to be more performant
