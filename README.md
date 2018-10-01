# Commander
Commander is a action runner that is little more than a `map[string]func()`.

Commander manages the lifecycle of in order execution of nonblocking, but long running
functions that need safe for concurrent access to a global state.

This state is represented as a named `*interface{}` type called commander.Config.


Commander provides tools to organize an action library that can be overridden, extended, erased,
and searched, without using reflect, or unsafe.


It is uses only the go standard library, and is pure go.


Originally, commander was designed to be a terminal read eval print loop.
After working through some of the design, I realized it was more useful to keep
commander "headless"  and let it be used as an engine that can have a head attached
by some other library decided by the user.

### Goal
Commanders goal is to unify and simplify a process of defining and executing work
that requires access to the calling thread, while supporting nonblocking, long running,
and in order execution of functions that can mutate a global state.



## STATUS (NOT READY, ALPHA)
The api will no longer have any major name changes, and will continue to be tested
internaly.  Most actions work, but some are still untested. 

USEAGE:


```go
package main

import (
    "fmt"
    cmd "github.com/iamneal/commander"
)

func main() {
    // the mystate could be anything.  Your actions will be give this during action processing 
    //The Commands struct does not mutate or read this object, it just delivers it.
    mystate := SomeBigCrazyThing{}

    // cast the thing you are keeping track of to a cmd.Config.
    // Since cmd.Config is just the empty interface, everything will be castable
    config := cmd.Config(mystate)

    // make a new commands struct with a pointer to the config 
    commands := cmd.NewCommands(&config))

    // set addtional actions in the command map by calling Set
    // the key will be the action name
    commands.Set(cmd.PrintAction("hello", "HELLO WORLD"))

    // Commander supports a pretty robust Builder pattern to build, split, and override actions
    // more documentation and examples of its use coming soon
    commands.Set(cmd.Build().WithNameV("goodbye").WithVoidExecuteVoid(func(c *cmd.Config) error {
        fmt.Println("goodbye world")
    }))

    for {
        var actionName string
        fmt.Scanln("what command to run? ", &actionName)

        // command is ran here. If the command name is not found, the 
        // help command is ran. Commands are run in parallel, but it 
        // is safe to treat them as having sole access to the config
        // for the length of the action. 
        // the returned "Work"  will be populated when the work is done.
        if err := commands.Get(actionName)(); err != nil {
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

### Actions
Actions are separated into two parts:
- Payload creation
    - this takes place on the calling thread
    - the `commander.Payload` type is the function
    - while not (yet) enforced these should not mutate the provided Config.
    - Payload functions should return a payload and error this is passed to the
    Execute function.
    - If an Execution should be skipped, the payload function should return:
    `commander.Skip`.  The Execute will not be sent to the work queue.
- Execution
    - Work is created from the Execution function and the previous steps payload
    - the work is queued up to happen in the background thread.
    - a goroutine is started that will remove the action's removals,
    and add the action's additions when the work is done.
    - the work is returned to the calling thread, it can be waited on their

### Default Commands
These commands come with every initialized Commands object.

They can be removed by calling `.Remove(keynames...)` if they are not
wanted, or overriden with `.Set(...)`


- print-config
    - pretty print the config given to the Commands instance
- tags
    - list all the currently known tags in this Commands instance
- last
    - pretty print the last action ran, and its result if it is finished
- lookup
    -  runs like last, but prompts for an action name as input, and pretty
    prints the last action of that name
- quit
    - returns `commander.Quit` which is an instance of `commander.QuitError`
    - no use on its own, but useful in loops that check for use input
- filter
    - prompts for a tag name, then lists all actions that are tagged with that tag name
- aliases
    - show aliases to an action
- help
    - prints all known actions
- save
    - save the pretty json of this isntance's Config object to a file
- load
    - override this instance's Config with one loaded from the prompted file

### Other useful actions
- Watch
    - watches a child action, by:
        - first building a payload on the calling thread
        - then every (configureable) seconds, the child action's execute
        function is called with the same payload every tick.
    - a `stop-(childname)` function is setup where `(childname)` is the
    the child function passed to watch's name. If this action is executed, the watch stops.

### Bazel integration
One benefit of having a library that doens't import anything out of the standard lib, is
I can write template binaries that import code, without fear of an import cycle.

a `rules_commander.bzl` script will provide rules that will: 
- generate/run commander applications that import/use your custom action libraries.
- generate action libraries that can be imported into your commander applications


## Immediate TODO list
- create a roadmap, not just a todo list.
- update the processor internals to be more performant
- Execute, and Payload sub-interfaces that can asserted on internally for additional functionality.
    - an example of this would be something like an `ExecuteReadOnly` interface that doesn't need
    to be put into the work queue.
    - another example would be `PayloadCmdlineProcessor` that could optionally be given
    the extra `[]string` recieved from processing a fmt.Scanln, for command line processing.
- Named opts that can extend/change functionality in the Commands struct
- Add a describe default command that will return the action's descriptions



    


