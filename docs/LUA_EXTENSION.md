# Extending Lua Functions in Go

This document explains how to add new Lua-callable functions to the
extractor's Go code. All registrations happen in `registerFuncs()`
in `pkg/extractor/extractor.go`.

## Adding a New Function

Each function is registered with `L.SetGlobal(name, L.NewFunction(...))`.

### Simple Example: Get Page Title

```go
L.SetGlobal("page_title", L.NewFunction(func(L *lua.LState) int {
    // Read arguments
    // (none in this case)

    // Do work
    title := page.MustInfo().Title

    // Push return value(s)
    L.Push(lua.LString(title))
    return 1 // number of return values
}))
```

In Lua:
```lua
local title = page_title()
log(title)
```

### Example with Arguments and Error Handling

```go
L.SetGlobal("get_attribute", L.NewFunction(func(L *lua.LState) int {
    // Required string argument
    selector := L.CheckString(1)
    // Optional string argument with default
    attr := L.OptString(2, "src")

    el, err := page.Element(selector)
    if err != nil {
        // Return nil + error message
        L.Push(lua.LNil)
        L.Push(lua.LString(err.Error()))
        return 2
    }

    val, _ := el.Attribute(attr)
    if val == nil {
        L.Push(lua.LNil)
        return 1
    }
    L.Push(lua.LString(*val))
    return 1
}))
```

In Lua:
```lua
local src = get_attribute("iframe", "src")
if src then
    log("Iframe src: " .. src)
else
    log("No iframe found")
end
```

## Important Notes

- **Always return the correct number of values** in `return N`.
- **On error, return `nil, error_string`** with `return 2`.
- **On success with a value**, push the value and `return 1`.
- **On success with no value**, just `return 0`.
- Use `L.CheckString(n)` for required string params (panics if missing).
- Use `L.OptString(n, default)` for optional string params.
- Use `L.CheckNumber(n)` / `L.OptNumber(n, default)` for numbers.
- Use `L.CheckInt(n)` / `L.OptInt(n, default)` for integers.
- Use `L.CheckBool(n)` / `L.OptBool(n, default)` for booleans.

## Page Access

The `page *rod.Page` variable is available in the closure of each
function registered in `registerFuncs`. You can call any rod method:

- `page.Element(selector)` - find an element
- `page.Elements(selector)` - find all matching elements
- `page.Eval(js)` - evaluate JS
- `page.MustHTML()` - get page HTML
- `page.Mouse.MustMoveTo(x, y)` - move mouse
- `page.Timeout(d).Element(sel)` - wait for element with timeout

## Adding to the Lua State

All functions must be registered inside `registerFuncs(L, page)`.
The `L` is `*lua.LState` and the `page` is `*rod.Page`. Both are
available in the closure.

## Testing New Functions

After adding a function, test it with a simple Lua script:

```lua
-- test.lua
function on_ready()
  local title = page_title()
  log("Title: " .. title)
end
```

Run the extractor with a URL and this script. Check the output for errors.
