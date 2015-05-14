// Copyright (c) 2015 tree-server contributors

package engine

import (
	"github.com/layeh/gopher-luar"
	"github.com/yuin/gopher-lua"
)

// Engine struct stores a pointer to a lua.LState providing a simplified API.
type Engine struct {
	state *lua.LState
}

// ScriptFunction is a type alias for a function that receives an Engine and
// returns an int.
type ScriptFunction func(*Engine) int

// ScriptFnMap is a type alias for map[string]ScriptFunction
type ScriptFnMap map[string]ScriptFunction

// Create a new engine containing a new lua.LState.
func NewEngine() *Engine {
	return &Engine{
		state: lua.NewState(),
	}
}

// Close will perform a close on the Lua state.
func (e *Engine) Close() {
	e.state.Close()
}

// LoadFile runs the file through the Lua interpreter.
func (e *Engine) LoadFile(fn string) error {
	return e.state.DoFile(fn)
}

// LoadString runs the given string through the Lua interpreter.
func (e *Engine) LoadString(src string) error {
	return e.state.DoString(src)
}

// SetVal allows for setting global variables in the loaded code.
func (e *Engine) SetGlobal(name string, val interface{}) {
	v := e.ValueFor(val)

	e.state.SetGlobal(name, v.lval)
}

// GetGlobal returns the value associated with the given name, or LuaNil
func (e *Engine) GetGlobal(name string) *Value {
	lv := e.state.GetGlobal(name)

	return newValue(lv)
}

// SetField applies the value to the given table associated with the given
// key.
func (e *Engine) SetField(tbl *Value, key string, val interface{}) {
	v := e.ValueFor(val)
	e.state.SetField(tbl.lval, key, v.lval)
}

// RegisterFunc registers a Go function with the script. Using this method makes
// Go functions accessible through Lua scripts.
func (e *Engine) RegisterFunc(name string, fn interface{}) {
	var lfn lua.LValue
	if sf, ok := fn.(func(*Engine) int); ok {
		lfn = e.genScriptFunc(sf)
	} else {
		v := e.ValueFor(fn)
		lfn = v.lval
	}
	e.state.SetGlobal(name, lfn)
}

// RegisterModule registers a Go module with the Engine for use within Lua.
func (e *Engine) RegisterModule(name string, loadFn func(*Engine) *Value) {
	loader := func(l *lua.LState) int {
		e := &Engine{l}
		mod := loadFn(e)
		e.PushRet(mod)

		return 1
	}

	e.state.PreloadModule(name, loader)
}

// GenerateModule returns a table that has been loaded with the given script
// function map.
func (e *Engine) GenerateModule(fnMap ScriptFnMap) *Value {
	tbl := e.state.NewTable()
	realFnMap := make(map[string]lua.LGFunction)
	for k, fn := range fnMap {
		realFnMap[k] = e.wrapScriptFunction(fn)
	}

	mod := e.state.SetFuncs(tbl, realFnMap)

	return newValue(mod)
}

// PopArg returns the top value on the Lua stack.
// This method is used to get arguments given to a Go function from a Lua script.
// This method will return a Value pointer that can then be converted into
// an appropriate type.
func (e *Engine) PopArg() *Value {
	lv := e.state.Get(-1)
	e.state.Pop(1)

	return newValue(lv)
}

// PushRet pushes the given Value onto the Lua stack.
// Use this method when 'returning' values from a Go function called from a
// Lua script.
func (e *Engine) PushRet(val interface{}) {
	v := e.ValueFor(val)
	e.state.Push(v.lval)
}

// PopBool returns the top of the stack as an actual Go bool.
func (e *Engine) PopBool() bool {
	v := e.PopArg()

	return v.AsBool()
}

// PopFunction is an alias for PopArg, provided for readability when specifying
// the desired value from the top of the stack.
func (e *Engine) PopFunction() *Value {
	return e.PopArg()
}

// PopInt returns the top of the stack as an actual Go int.
func (e *Engine) PopInt() int {
	v := e.PopArg()
	i := int(v.AsNumber())

	return i
}

// PopInt64 returns the top of the stack as an actual Go int64.
func (e *Engine) PopInt64() int64 {
	v := e.PopArg()
	i := int64(v.AsNumber())

	return i
}

// PopFloat returns the top of the stack as an actual Go float.
func (e *Engine) PopFloat() float64 {
	v := e.PopArg()

	return v.AsFloat()
}

// PopNumber is an alias for PopArg, provided for readability when specifying
// the desired value from the top of the stack.
func (e *Engine) PopNumber() *Value {
	return e.PopArg()
}

// PopString returns the top of the stack as an actual Go string value.
func (e *Engine) PopString() string {
	v := e.PopArg()

	return v.AsString()
}

// PopTable is an alias for PopArg, provided for readability when specifying
// the desired value from the top of the stack.
func (e *Engine) PopTable() *Value {
	return e.PopArg()
}

// PopInterface returns the top of the stack as an actual Go interface.
func (e *Engine) PopInterface() interface{} {
	v := e.PopArg()

	return v.Interface()
}

// Call allows for calling a method by name.
// The second parameter is the number of return values the function being
// called should return. These values will be returned in a slice of Value
// pointers.
func (e *Engine) Call(name string, retCount int, params ...interface{}) ([]*Value, error) {
	luaParams := make([]lua.LValue, len(params))
	for i, iface := range params {
		v := e.ValueFor(iface)
		luaParams[i] = v.lval
	}

	err := e.state.CallByParam(lua.P{
		Fn:      e.state.GetGlobal(name),
		NRet:    retCount,
		Protect: true,
	}, luaParams...)

	if err != nil {
		return nil, err
	}

	retVals := make([]*Value, retCount)
	for i := 0; i < retCount; i++ {
		retVals[i] = newValue(e.state.Get(-1))
	}
	e.state.Pop(retCount)

	return retVals, nil
}

// DefineType creates a construtor with the given name that will generate the
// given type.
func (e *Engine) DefineType(name string, val interface{}) {
	cons := luar.NewType(e.state, val)
	e.state.SetGlobal(name, cons)
}

// ValueFor takes a Go type and creates a lua equivalent Value for it.
func (e *Engine) ValueFor(val interface{}) *Value {
	if v, ok := val.(*Value); ok {
		return v
	} else {
		return newValue(luar.New(e.state, val))
	}
}

// LuaTable creates and returns a new LuaTable.
func (e *Engine) LuaTable() *Value {
	return newValue(e.state.NewTable())
}

// wrapScriptFunction turns a ScriptFunction into a lua.LGFunction
func (e *Engine) wrapScriptFunction(fn ScriptFunction) lua.LGFunction {
	return func(l *lua.LState) int {
		e := &Engine{state: l}

		return fn(e)
	}
}

// genScriptFunc will wrap a ScriptFunction with a function that gopher-lua
// expects to see when calling method from Lua.
func (e *Engine) genScriptFunc(fn ScriptFunction) *lua.LFunction {
	return e.state.NewFunction(e.wrapScriptFunction(fn))
}
