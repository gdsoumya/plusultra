package engine

import (
	"errors"
	"fmt"

	"go.starlark.net/resolve"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// EvalError is a script error at a source position.
type EvalError struct {
	File      string
	Line, Col int
	Msg       string
}

func (e *EvalError) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Col, e.Msg)
}

func toEvalError(file string, err error) *EvalError {
	at := func(pos syntax.Position, msg string) *EvalError {
		return &EvalError{File: file, Line: int(pos.Line), Col: int(pos.Col), Msg: msg}
	}
	if e, ok := errors.AsType[syntax.Error](err); ok {
		return at(e.Pos, e.Msg)
	}
	if list, ok := errors.AsType[resolve.ErrorList](err); ok {
		return at(list[0].Pos, list[0].Msg)
	}
	if e, ok := errors.AsType[*starlark.EvalError](err); ok {
		// The innermost frame is a builtin (line 0) when a builtin failed; report its caller.
		for i := range e.CallStack {
			if pos := e.CallStack.At(i).Pos; pos.Line > 0 {
				return at(pos, e.Msg)
			}
		}
		return &EvalError{File: file, Msg: e.Msg}
	}
	return &EvalError{File: file, Msg: err.Error()}
}
