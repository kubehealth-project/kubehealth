package builtins

import (
	_ "embed"
	"fmt"

	"github.com/kubehealth-project/kubehealth/api"
	kubehealthlua "github.com/kubehealth-project/kubehealth/lua"
)

//go:embed lua/statefulset.lua
var statefulSetLua string

var statefulSetCheck = mustLuaCheck(statefulSetLua)

func mustLuaCheck(script string) api.Check {
	check, err := kubehealthlua.NewCheck(script)
	if err != nil {
		panic(fmt.Sprintf("compile built-in Lua health check: %v", err))
	}
	return check
}
