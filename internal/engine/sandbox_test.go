package engine

import (
	"errors"
	"strings"
	"testing"
	"time"

	"go.starlark.net/starlark"
)

func execErr(t *testing.T, src string, predeclared starlark.StringDict) error {
	t.Helper()
	if _, err := exec(src, "test.star", predeclared); err != nil {
		return err
	}
	t.Fatalf("exec(%q) succeeded, want error", src)
	return nil
}

func assertContains(t *testing.T, err error, want string) {
	t.Helper()
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not mention %q", err, want)
	}
}

func TestExecRejectsWhile(t *testing.T) {
	assertContains(t, execErr(t, "while True: pass\n", nil), "while")
}

func TestExecRejectsRecursion(t *testing.T) {
	src := "def f(n):\n    return f(n - 1) if n else 0\nf(1)\n"
	assertContains(t, execErr(t, src, nil), "recursively")
}

func TestExecStepLimit(t *testing.T) {
	start := time.Now()
	err := execErr(t, "for i in range(1000000000): pass\n", nil)
	if elapsed := time.Since(start); elapsed >= 2*time.Second {
		t.Fatalf("step limit took %s", elapsed)
	}
	assertContains(t, err, "too many steps")
}

func TestExecTimeout(t *testing.T) {
	sleep := starlark.NewBuiltin("sleep", func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
		time.Sleep(evalTimeout + 100*time.Millisecond)
		return starlark.None, nil
	})
	assertContains(t, execErr(t, "sleep()\nx = 1\n", starlark.StringDict{"sleep": sleep}), "timeout")
}

func TestExecNoLoad(t *testing.T) {
	assertContains(t, execErr(t, `load("x", "y")`+"\n", nil), "load")
}

func TestExecAllowsTopLevelLoops(t *testing.T) {
	src := "n = 0\nfor i in range(3):\n    n += i\nfor i in range(3):\n    n += i\n"
	if _, err := exec(src, "test.star", nil); err != nil {
		t.Fatal(err)
	}
}

func TestEvalErrorHasLine(t *testing.T) {
	err := execErr(t, "a = 1\nb = 2\nc = )\n", nil)
	var ee *EvalError
	if !errors.As(err, &ee) || ee.Line != 3 {
		t.Fatalf("got %#v, want EvalError at line 3", err)
	}
	if !strings.HasPrefix(err.Error(), "test.star:3:") {
		t.Fatalf("error %q not formatted file:line:col: msg", err)
	}
}

func TestEvalErrorRuntimeLineSkipsBuiltins(t *testing.T) {
	err := execErr(t, "a = 1\nfail(\"boom\")\n", nil)
	var ee *EvalError
	if !errors.As(err, &ee) || ee.Line != 2 || !strings.Contains(ee.Msg, "boom") {
		t.Fatalf("got %#v, want EvalError at line 2 with message", err)
	}
}
