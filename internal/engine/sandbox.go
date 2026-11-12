// Package engine evaluates program scripts in a Starlark sandbox.
package engine

import (
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

const (
	maxSteps    = 1_000_000
	evalTimeout = time.Second
	timerKey    = "engine.timer"
)

// While and Recursion stay off. Top-level loops need TopLevelControl, and
// GlobalReassign lets two top-level loops reuse a loop variable.
var fileOptions = &syntax.FileOptions{Set: true, TopLevelControl: true, GlobalReassign: true}

// newThread returns a thread for one evaluation. It is limited to maxSteps and
// cancelled evalTimeout after creation. Load is nil, so load() fails.
func newThread(name string) *starlark.Thread {
	t := &starlark.Thread{
		Name:  name,
		Print: func(*starlark.Thread, string) {}, // the default writes to server stderr
	}
	t.SetMaxExecutionSteps(maxSteps)
	t.SetLocal(timerKey, time.AfterFunc(evalTimeout, func() { t.Cancel("timeout") }))
	return t
}

// exec runs src in the sandbox and returns its globals. Errors are *EvalError.
func exec(src, filename string, predeclared starlark.StringDict) (starlark.StringDict, error) {
	t := newThread(filename)
	defer t.Local(timerKey).(*time.Timer).Stop()
	globals, err := starlark.ExecFileOptions(fileOptions, t, filename, src, predeclared)
	if err != nil {
		return nil, toEvalError(filename, err)
	}
	return globals, nil
}
