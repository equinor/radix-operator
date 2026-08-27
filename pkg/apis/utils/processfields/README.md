# processfields

`WalkFields` walks every leaf field of a config struct and hands each one to a callback,
together with a setter that parses a string into that field.

The package holds no policy of its own. It does not read struct tags, does not know about
environment variables, and does not decide what "required" means — the callback does. That
is what lets a single traversal serve both validation and env overrides:

```go
// Validate
err := processfields.WalkFields(cfg, func(path string, field reflect.StructField, value reflect.Value, _ processfields.SetValFunc) error {
	if field.Tag.Get("required") == "true" && value.IsZero() {
		return fmt.Errorf("field %q is required but not set", path)
	}
	return nil
})

// Override from the environment
err := processfields.WalkFields(cfg, func(path string, field reflect.StructField, _ reflect.Value, setter processfields.SetValFunc) error {
	if v := os.Getenv(field.Tag.Get("env")); v != "" {
		return setter(v)
	}
	return nil
})
```

The root must be a struct or a non-nil pointer to one. Pass a **pointer** if you intend to
set anything — fields of a struct passed by value are not addressable and every setter will
fail with `cannot set field "X"`.

## Supported field types

| Type                                          | Parsed with                                                   |
| --------------------------------------------- | ------------------------------------------------------------- |
| `string`                                      | assigned as-is                                                |
| `bool`                                        | `strconv.ParseBool`                                           |
| `int`, `int8`, `int16`, `int32`, `int64`      | `strconv.ParseInt`, range-checked for the width               |
| `uint`, `uint8`, `uint16`, `uint32`, `uint64` | `strconv.ParseUint`                                           |
| `float32`, `float64`                          | `strconv.ParseFloat`                                          |
| `time.Duration`                               | `time.ParseDuration` — a unit is required, `"30"` is rejected |
| `encoding.TextUnmarshaler`                    | `UnmarshalText`                                               |
| `encoding.BinaryUnmarshaler`                  | `UnmarshalBinary`                                             |

Defined types built on those kinds work too, e.g. `type logLevel string`.

`time.Time` and `url.URL` are supported through the unmarshaler rows above, which is also
how you add support for a type of your own. When a type implements both interfaces, text
wins. The unmarshaler must have a **pointer receiver**; a value receiver would unmarshal
into a copy and lose the result, so it is rejected with an error rather than silently
ignored.

## Traversal

- Nested structs are recursed into, at any depth, in declaration order.
- Embedded structs are flattened — their fields appear as if declared on the outer struct,
  and contribute nothing to the error path. This includes embedded *unexported* struct
  types, whose exported fields are settable through reflection.
- Slices and arrays of structs (or of struct pointers) are walked element by element.
- Unexported fields are skipped.
- A `nil` pointer to a struct is allocated so its fields can be visited, and dropped again
  if it stayed empty. See [Optional sections](#optional-sections) below.
- Traversal stops at the first error returned by the callback or a setter.

Errors name the full path to the field, so identically named fields in different sections
stay distinguishable:

```
field "Operator.LogLevel": failed to parse int: strconv.ParseInt: parsing "abc": invalid syntax
field "Sections[1].Threads": failed to parse int: ...
```

The callback is given the same path as its first argument, so it can report errors of its
own the same way. Fields of embedded structs are flattened, and contribute nothing to the
path:

```go
type Config struct {
	Common             // Common.ClusterName is reported as "ClusterName"
	Operator  Operator // "Operator.LogLevel"
	Sections []Section // "Sections[0].Threads"
}
```

## Optional sections

A pointer to a struct models an optional section. Its fields are always visited — the
walker allocates the section, walks it, and then resets it to `nil` if it is still zero
when the walk finishes. Chains collapse bottom-up, so an outer section whose only content
was an empty inner section is dropped too.

Pointers to *leaf* types work the same way from the outside: `nil` unless something sets
them, and a failed parse leaves them `nil` rather than half-written.

```go
Timeout *time.Duration // nil unless set; stays nil if the value does not parse
Section *Section       // nil unless something inside it ends up non-zero
```

Two consequences follow from deciding this on the *value* rather than on whether a setter
ran:

- **Writing a zero value drops the section.** Setting `Enabled` to `false` or `Threads` to
  `0` inside an otherwise empty section leaves it looking untouched, so it is discarded. If
  a section needs a meaningful `false`, give it a `*bool` or a non-zero default.
- **A setter captured for later may not reach the config.** Setters are valid after the
  walk returns, but one belonging to a dropped section writes into a struct that is no
  longer referenced: it reports success and changes nothing. Call setters during the walk.

Give a section its own `UnmarshalText` if you want it treated as a leaf instead.

## Not supported

- **Slices and maps of non-struct types** — `[]string`, `map[string]string`. They are
  visited, but the setter fails with `unsupported field type: slice`.
- **Pointer to slice** — `*[]Section` is neither a nested struct nor a list, so it falls
  through to the leaf path and errors.
- **Interface, channel, function and complex fields.**
- **Recursive types** — a struct that reaches itself is rejected with
  `recursive type ... is not supported` rather than looped over.
- **Defined types declared from `time.Duration`** — `type timeout time.Duration` is parsed
  as a plain integer, not a duration. Reflection cannot tell it apart from any other
  defined `int64`, so `"5m30s"` is rejected. Implement `TextUnmarshaler` if you need it.
- **Value-receiver unmarshalers** — rejected with an error, see above.
- **Struct tags** — nothing here interprets them; that is the caller's job.

## Notes

Everything is resolved through reflection on every call, with no type cache. It is built
for parsing configuration once at startup, not for hot paths.
