package engine

import (
	"encoding/json"
	"fmt"
	"math/big"
	"slices"
	"strconv"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

// overlayJSON is an overlay with its state values in JSON form. Its State
// shadows the embedded one.
type overlayJSON struct {
	model.Overlay
	State map[model.ProgKey]map[string]stateJSON
}

// MarshalOverlay encodes o as JSON for storage.
func MarshalOverlay(o model.Overlay) ([]byte, error) {
	j := overlayJSON{Overlay: o, State: make(map[model.ProgKey]map[string]stateJSON, len(o.State))}
	for k, vars := range o.State {
		j.State[k] = make(map[string]stateJSON, len(vars))
		for name, v := range vars {
			j.State[k][name] = stateJSON{v}
		}
	}
	return json.Marshal(j)
}

// UnmarshalOverlay decodes what MarshalOverlay encoded. State values come back frozen.
func UnmarshalOverlay(b []byte) (model.Overlay, error) {
	var j overlayJSON
	if err := json.Unmarshal(b, &j); err != nil {
		return model.Overlay{}, fmt.Errorf("overlay: %w", err)
	}
	o := j.Overlay
	for k, vars := range j.State {
		if o.State == nil {
			o.State = make(map[model.ProgKey]map[string]starlark.Value, len(j.State))
		}
		o.State[k] = make(map[string]starlark.Value, len(vars))
		for name, v := range vars {
			o.State[k][name] = v.Value
		}
	}
	return o, nil
}

// stateJSON is a state value in JSON: None, bools and strings as themselves,
// anything else as a one-key object naming its kind. Numbers are strings so
// ints keep any size and floats keep NaN and the infinities.
type stateJSON struct{ starlark.Value }

func (s stateJSON) MarshalJSON() ([]byte, error) {
	x, err := encodeState(s.Value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(x)
}

func (s *stateJSON) UnmarshalJSON(b []byte) error {
	var x any
	if err := json.Unmarshal(b, &x); err != nil {
		return err
	}
	v, err := decodeState(x)
	if err != nil {
		return err
	}
	v.Freeze()
	s.Value = v
	return nil
}

func encodeState(v starlark.Value) (any, error) {
	kind := func(k string, x any) (any, error) { return map[string]any{k: x}, nil }
	switch v := v.(type) {
	case starlark.NoneType:
		return nil, nil
	case starlark.Bool:
		return bool(v), nil
	case starlark.String:
		return string(v), nil
	case starlark.Int:
		return kind("int", v.String())
	case starlark.Float:
		return kind("float", strconv.FormatFloat(float64(v), 'g', -1, 64))
	case *weightValue:
		return kind("weight", []any{v.num(), string(v.unit)})
	case *pctValue:
		return kind("pct", v.num())
	case *bwValue:
		return kind("bw", []any{v.num(), string(v.unit)})
	case *starlark.List, starlark.Tuple:
		xs, err := encodeAll(slices.Collect(starlark.Elements(v.(starlark.Iterable))))
		if err != nil {
			return nil, err
		}
		return kind(v.Type(), xs)
	case *starlark.Dict:
		pairs := make([]any, 0, v.Len())
		for _, kv := range v.Items() {
			p, err := encodeAll(kv[:])
			if err != nil {
				return nil, err
			}
			pairs = append(pairs, p)
		}
		return kind("dict", pairs)
	}
	return nil, fmt.Errorf("state cannot hold %s", v.Type())
}

func encodeAll(vs []starlark.Value) ([]any, error) {
	out := make([]any, len(vs))
	for i, v := range vs {
		x, err := encodeState(v)
		if err != nil {
			return nil, err
		}
		out[i] = x
	}
	return out, nil
}

func decodeState(x any) (starlark.Value, error) {
	switch x := x.(type) {
	case nil:
		return starlark.None, nil
	case bool:
		return starlark.Bool(x), nil
	case string:
		return starlark.String(x), nil
	case map[string]any:
		if len(x) == 1 {
			for k, v := range x {
				return decodeKind(k, v)
			}
		}
	}
	return nil, fmt.Errorf("bad state value %v", x)
}

func decodeKind(kind string, x any) (starlark.Value, error) {
	num := func(x any) (float64, bool) {
		s, ok := x.(string)
		f, err := strconv.ParseFloat(s, 64)
		return f, ok && err == nil
	}
	pair := func() (float64, model.Unit, bool) {
		xs, ok := x.([]any)
		if !ok || len(xs) != 2 {
			return 0, "", false
		}
		f, ok := num(xs[0])
		u, isStr := xs[1].(string)
		return f, model.Unit(u), ok && isStr
	}
	switch kind {
	case "int":
		s, _ := x.(string)
		if n, ok := new(big.Int).SetString(s, 10); ok {
			return starlark.MakeBigInt(n), nil
		}
	case "float":
		if f, ok := num(x); ok {
			return starlark.Float(f), nil
		}
	case "pct":
		if f, ok := num(x); ok {
			return &pctValue{quantity{v: f}}, nil
		}
	case "weight":
		if f, u, ok := pair(); ok {
			return &weightValue{quantity{f, u}}, nil
		}
	case "bw":
		if f, u, ok := pair(); ok {
			return &bwValue{quantity{f, u}}, nil
		}
	case "list", "tuple", "dict":
		xs, ok := x.([]any)
		if !ok {
			break
		}
		if kind == "dict" {
			return decodeDict(xs)
		}
		vs := make([]starlark.Value, len(xs))
		for i, x := range xs {
			v, err := decodeState(x)
			if err != nil {
				return nil, err
			}
			vs[i] = v
		}
		if kind == "tuple" {
			return starlark.Tuple(vs), nil
		}
		return starlark.NewList(vs), nil
	}
	return nil, fmt.Errorf("bad %s state value %v", kind, x)
}

func decodeDict(pairs []any) (starlark.Value, error) {
	d := starlark.NewDict(len(pairs))
	for _, p := range pairs {
		kv, ok := p.([]any)
		if !ok || len(kv) != 2 {
			return nil, fmt.Errorf("bad dict state entry %v", p)
		}
		k, err := decodeState(kv[0])
		if err != nil {
			return nil, err
		}
		v, err := decodeState(kv[1])
		if err != nil {
			return nil, err
		}
		if err := d.SetKey(k, v); err != nil {
			return nil, err
		}
	}
	return d, nil
}
