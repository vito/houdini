package tools

import (
	"strings"
	"os"
)

type annotatedError struct {
	error
	components []string
}

func mergePrevious(current, previous []string) []string {
	if len(previous) > 0 && previous[0] == current[len(current) - 1] {
		previous = previous[1:]
	}
	return append(current, previous...)
}

/*
Create a nested component error. Will take err, and create a new one with the list of components
that's supposed to tell you where it has occured.
 */
func NewError(err error, c ...string) error {
	if err != nil {
		var result annotatedError

		switch e := err.(type) {
		case nil:
			return nil
		case *annotatedError:
			result.error, result.components = e.error, mergePrevious(c, e.components)
		case *os.SyscallError:
			result.error, result.components = e.Err, mergePrevious(c, []string{"syscall:" + e.Syscall,})
		default:
			result.error, result.components = err, c
		}
		return &result
	}
	return err
}

func (e *annotatedError) Error() string {
	return strings.Join(e.components, "/") + ": " + e.error.Error()
}

func GetAnnotations(err error) []string {
	if err != nil {
		if e, ok := err.(*annotatedError); ok {
			return e.components
		}
	}
	return nil
}

func HasAnnotation(err error, component string) bool {
	if c := GetAnnotations(err); len(c) != 0 {
		for _, v := range c {
			if component == v {
				return true
			}
		}
	}
	return false
}

type ErrorContext string

func (c ErrorContext) NewError(err error, s ...string) error {
	return NewError(err, append([]string{string(c),}, s...)...)
}
