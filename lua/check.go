// Package lua adapts Lua scripts to KubeHealth resource checks.
package lua

import (
	"context"
	"fmt"
	"time"

	luavm "github.com/yuin/gopher-lua"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubehealth-project/kubehealth/api"
)

const defaultTimeout = 500 * time.Millisecond

type config struct {
	timeout time.Duration
}

// Option configures a Lua health check.
type Option func(*config)

// WithTimeout limits each execution of the Lua script.
func WithTimeout(timeout time.Duration) Option {
	return func(config *config) {
		config.timeout = timeout
	}
}

// Registrar accepts a health check for an exact GVK.
type Registrar interface {
	Register(schema.GroupVersionKind, api.Check) error
}

// Register creates a Lua-backed check and registers it for an exact GVK.
func Register(registrar Registrar, gvk schema.GroupVersionKind, script string, options ...Option) error {
	check, err := NewCheck(script, options...)
	if err != nil {
		return err
	}
	return registrar.Register(gvk, check)
}

// NewCheck creates a KubeHealth check from a Lua script. The script receives
// the resource as the global table "obj" and must return an assessment table.
func NewCheck(script string, options ...Option) (api.Check, error) {
	configuration := config{timeout: defaultTimeout}
	for _, option := range options {
		if option != nil {
			option(&configuration)
		}
	}
	if configuration.timeout <= 0 {
		return nil, fmt.Errorf("Lua health check timeout must be positive")
	}
	if script == "" {
		return nil, fmt.Errorf("Lua health check script must not be empty")
	}

	state := newState()
	_, err := state.LoadString(script)
	state.Close()
	if err != nil {
		return nil, fmt.Errorf("compile Lua health check: %w", err)
	}

	return func(obj *unstructured.Unstructured) (api.Assessment, error) {
		return execute(script, configuration.timeout, obj)
	}, nil
}

func execute(script string, timeout time.Duration, obj *unstructured.Unstructured) (api.Assessment, error) {
	state := newState()
	defer state.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	state.SetContext(ctx)
	state.SetGlobal("obj", toLuaValue(state, obj.Object))

	function, err := state.LoadString(script)
	if err != nil {
		return api.Assessment{}, fmt.Errorf("compile Lua health check: %w", err)
	}
	if err := state.CallByParam(luavm.P{Fn: function, NRet: 1, Protect: true}); err != nil {
		return api.Assessment{}, fmt.Errorf("execute Lua health check: %w", err)
	}

	result := state.Get(-1)
	table, ok := result.(*luavm.LTable)
	if !ok {
		return api.Assessment{}, fmt.Errorf("Lua health check must return a table, got %s", result.Type())
	}
	return decodeAssessment(table)
}

func newState() *luavm.LState {
	state := luavm.NewState(luavm.Options{SkipOpenLibs: true})
	for _, library := range []struct {
		name string
		open luavm.LGFunction
	}{
		{name: luavm.BaseLibName, open: luavm.OpenBase},
		{name: luavm.TabLibName, open: luavm.OpenTable},
		{name: luavm.StringLibName, open: luavm.OpenString},
		{name: luavm.MathLibName, open: luavm.OpenMath},
	} {
		state.Push(state.NewFunction(library.open))
		state.Push(luavm.LString(library.name))
		state.Call(1, 0)
	}

	for _, name := range []string{"dofile", "loadfile", "load", "require", "module"} {
		state.SetGlobal(name, luavm.LNil)
	}
	return state
}

func toLuaValue(state *luavm.LState, value any) luavm.LValue {
	switch typed := value.(type) {
	case nil:
		return luavm.LNil
	case bool:
		return luavm.LBool(typed)
	case string:
		return luavm.LString(typed)
	case int:
		return luavm.LNumber(typed)
	case int32:
		return luavm.LNumber(typed)
	case int64:
		return luavm.LNumber(typed)
	case float32:
		return luavm.LNumber(typed)
	case float64:
		return luavm.LNumber(typed)
	case map[string]any:
		table := state.NewTable()
		for key, item := range typed {
			table.RawSetString(key, toLuaValue(state, item))
		}
		return table
	case []any:
		table := state.CreateTable(len(typed), 0)
		for _, item := range typed {
			table.Append(toLuaValue(state, item))
		}
		return table
	default:
		return luavm.LString(fmt.Sprint(typed))
	}
}
