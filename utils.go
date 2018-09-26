package commander

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
)

func retry(times int, f func() error) error {
	var err error
	for i := 0; i < times; i++ {
		if err = f(); err == nil {
			return nil
		}
	}
	return err
}

func retrier(times int, f func() error) func() error {
	return func() error {
		return retry(times, f)
	}
}

type ErrChain struct {
	err error
}

func E() *ErrChain { return &ErrChain{} }

func (e *ErrChain) Then(f func() error) {
	e.Wrap(f)()
}

func (e *ErrChain) Wrap(f func() error) func() {
	return func() { e.WrapErr(f)() }
}

func (e *ErrChain) WrapAssign(f func(*error)) func() {
	return func() {
		if e.err == nil {
			f(&e.err)
		}
	}
}

func (e *ErrChain) WrapInt(f func() (int, error)) func() int {
	return func() int {
		var i int
		if e.err == nil {
			i, e.err = f()
		}
		return i
	}
}
func (e *ErrChain) WrapString(f func() (string, error)) func() string {
	return func() string {
		var i string
		if e.err == nil {
			i, e.err = f()
		}
		return i
	}
}

func (e *ErrChain) WrapErr(f func() error) func() error {
	return func() error {
		if e.err == nil {
			e.err = f()
		}
		return e.err
	}
}

// Assign lets you assign to this error by dereferencing the pointer
func (e *ErrChain) Assign() *error {
	return &e.err
}
func (e *ErrChain) Err() error {
	return e.err
}

func prompt(argName string) string {
	return fmt.Sprintf("%s | enter: ", argName)
}

// it would be cool if we could split this on space, and all the additional strings could be used as default arguments
// TODO move scan to the commands interface so it can run commands in the middle of a question for lists and the like
func scan(question string, pointer interface{}) error {
	fmt.Printf("%s\n>>> ", question)
	if _, err := fmt.Scanln(pointer); err != nil && !strings.Contains(err.Error(), "unexpected newline") {
		return err
	}
	return nil
}

func scanIntOrDefault(msg string, def int64) (out int64) {
	var err error
	var temp string

	out = def
	err = scan(msg, &temp)
	if err == nil {
		out, err = strconv.ParseInt(temp, 10, 64)
	}
	if err != nil {
		fmt.Printf("error scanning: %v\n", err)
	}
	return
}

func scanner(q string, pointer interface{}) func() error {
	return func() error { return scan(q, pointer) }
}
func prettyB(b []byte) []byte {
	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "  "); err != nil {
		fmt.Printf("error pretty printing: \n%v\n, returing original", err)
		return b
	}
	return out.Bytes()
}

func prettyS(s string) string {
	return string(prettyB([]byte(s)))
}
func prettyJ(data interface{}) string {
	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(data); err != nil {
		return fmt.Sprintf("{\n  \"error\": \"%s\"\n}", err.Error())
	}
	return buffer.String()
}

// prettify the data, if it inst able to be encoded,
// it returns a json string with an error field, and string message
func PrettyJson(data interface{}) string { return prettyJ(data) }

func dashes(s string) {
	fmt.Printf("---------%s---------\n", s)
}

func ReplaceHome(s *string) {
	if s != nil && len(*s) > 0 && (*s)[0] == '~' {
		user, err := user.Current()
		if err != nil {
			fmt.Printf("error replacing ~ in \n%s\n error: %v\n", *s, err)
			return
		}
		*s = path.Clean(path.Join(user.HomeDir, (*s)[1:]))
	}
}
func ReplaceDotSlash(s *string) {
	if s != nil && len(*s) > 1 && (*s)[0:2] == "./" {
		dir, err := os.Getwd()
		if err != nil {
			fmt.Printf("error replacing ./ in \n%s\n error: %v", s, err)
			return
		}
		*s = path.Clean(path.Join(dir, (*s)[2:]))
	}
}
