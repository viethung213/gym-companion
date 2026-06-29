---
name: uber-go-style
description: MANDATORY coding rules for Go. This rule set must be strictly enforced at all times when writing, modifying, or reviewing Go code (.go files) in this workspace. No exceptions are allowed.
---

# Uber Go Coding Rules & Standards (MANDATORY)

This document defines the mandatory coding rules and standards for all Go development in this workspace. Every line of Go code written, edited, or reviewed must strictly conform to these rules at all times.

---

## 1. Code Construction Guidelines

### Pointers to Interfaces
- **Rule**: Never use a pointer to an interface. Pass interfaces as values.
- **Example**:
  ```go
  // Good
  func handle(r io.Reader) { ... }

  // Bad
  func handle(r *io.Reader) { ... }
  ```

### Interface Compliance Checks
- **Rule**: Declare explicit, compile-time compliance checks for types implementing critical or public interfaces.
- **Example**:
  ```go
  type Handler struct{}
  var _ http.Handler = (*Handler)(nil)
  ```

### Receivers and Interfaces
- **Rule**: If in doubt, use pointer receivers. However:
  - If the receiver is a map, func, or chan, use a value receiver.
  - If the method must mutate state, the receiver must be a pointer.
  - If the struct contains a mutex or sync primitives, the receiver must be a pointer to avoid copying it.
  - If the struct is large, use a pointer receiver to avoid copying overhead.

### Interface Design
- **Rule**: Keep interfaces small (ideally 1-3 methods).
- **Rule**: Define interfaces close to where they are consumed (at the site of use/consumer side) rather than where the implementation is defined, promoting loose coupling.

### Context Parameter
- **Rule**: Always pass `context.Context` as the first parameter of a function or method, typically named `ctx`. Do not pass it inside a struct or as a middle/optional argument.
- **Example**:
  ```go
  func FetchData(ctx context.Context, id string) (*Data, error) { ... }
  ```

### Avoid Mutable Globals & Singletons
- **Rule**: Avoid package-level mutable global variables and singletons. Use constructors (`NewXxx`) and Dependency Injection (DI) to pass dependencies explicitly, which improves testability and avoids global state side effects.

### Mutexes
- **Rule**: Zero-value of `sync.Mutex` and `sync.RWMutex` is valid. Do not use pointers to mutexes.
- **Rule**: Never embed mutexes in structs (even unexported ones) as they leak implementation details. Keep them as private fields:
  ```go
  type client struct {
      mu sync.Mutex
      // ...
  }
  ```

### Slice and Map Boundaries
- **Rule**: Copy slices and maps when receiving them as parameters or returning them to protect internal state from modification:
  ```go
  func (d *Driver) SetTrips(trips []Trip) {
      d.trips = make([]Trip, len(trips))
      copy(d.trips, trips)
  }
  ```

### Defer
- **Rule**: Always use `defer` to clean up resources (locks, files, sockets) to ensure clean exits on multiple return paths.

### Channel Size
- **Rule**: Channel buffer sizes must be **zero** (unbuffered) or **one**. Any other size requires extreme justification and documentation.

### Enums
- **Rule**: Start enums at `1` using `iota + 1` unless the zero-value represents the desired default behavior.

### Time Handling
- **Rule**: Always use the `"time"` package. Use `time.Time` for instants, `time.Duration` for periods. If field naming requires unit representation in serialized data (e.g. JSON), include the unit suffix (e.g. `timeoutMillis`).

### Avoid init()
- **Rule**: Avoid `init()` where possible. If unavoidable, code in `init()` must be deterministic, avoid I/O, and must not depend on `init()` ordering or manipulate global/environment state. Never start goroutines in `init()`.

### Exit in Main
- **Rule**: Call `os.Exit` or `log.Fatal` only in `main()`. All other functions must return errors.
- **Rule**: Exit at most once in `main()`. Delegate the actual program logic to a separate function (e.g., `run()`) that returns an error, then handle the exit in `main()`.

---

## 2. Error Handling

### Error Matching & Naming
- **Static Errors**: Use `errors.New` with top-level package variables prefixed with `Err` (exported) or `err` (unexported).
- **Dynamic Errors**: Use custom error types with suffix `Error` (e.g., `type NotFoundError struct`).
- **Matching**: Use `errors.Is` for static errors and `errors.As` for custom error types.

### Error Wrapping & Propagation
- **Rule**: Use `%w` to wrap errors unless the underlying error should be hidden from callers, in which case use `%v`.
- **Rule**: Avoid redundant "failed to" prefixes in wrapped errors.
- **Example**:
  ```go
  // Good
  return fmt.Errorf("open file %q: %w", name, err)

  // Bad
  return fmt.Errorf("failed to open file %q: %w", name, err)
  ```

### Handle Errors Exactly Once
- **Rule**: Do not log an error and then return it. Log the error OR return the error.

### Type Assertions
- **Rule**: Always use the "comma ok" syntax to avoid runtime panics:
  ```go
  val, ok := obj.(string)
  if !ok {
      // handle error gracefully
  }
  ```

### Don't Panic
- **Rule**: Code running in production must avoid `panic`. If an error occurs, return it to the caller.
- **Exception**: Panics are allowed only during program initialization for irrecoverable errors (e.g., `template.Must`), or when a truly irrecoverable state (like a nil pointer dereference) is reached. In tests, use `t.Fatal` instead of panics.

---

## 3. Concurrency & Goroutines

### Lifetime Management
- **Rule**: Never use "fire-and-forget" goroutines. Every goroutine must have a predictable stop time or a mechanism to signal it to stop.
- **Rule**: Wait for goroutines to exit when shutting down using `sync.WaitGroup` or a coordination channel.
- **Rule**: Never start goroutines in `init()`. Instead, expose a manager struct with startup/shutdown controls.

### Use go.uber.org/atomic
- **Rule**: Use `go.uber.org/atomic` instead of `sync/atomic` for raw atomic operations to ensure type-safety (e.g., `atomic.Bool`) and avoid data races caused by forgetting atomic operations.

---

## 4. Performance Guidelines

### Primitives to Strings
- **Rule**: Use the `strconv` package instead of `fmt` (e.g. `strconv.Itoa` instead of `fmt.Sprint`).

### String/Byte Conversions
- **Rule**: Avoid repeated string-to-byte conversions in hot paths. Do the conversion once and capture it.

### Capacity Specification
- **Rule**: Preallocate slices and maps with capacity or size hints wherever size is predictable:
  ```go
  items := make([]string, 0, len(source))
  m := make(map[string]int, len(source))
  ```

---

## 5. Style & Formatting

- **Line Length**: Soft limit of **99 characters**.
- **Declaration Grouping**: Group related variables, constants, imports, and types in `(...)` blocks.
- **Imports**: Group imports into standard library first, followed by third-party packages, separated by a blank line.
- **Package Names**: Short, lowercase, singular, and meaningful (avoid `util`, `helper`, `shared`).
- **Function Names**: `MixedCaps` (camelCase or PascalCase).
- **Initialisms**: Consistent with standard Go style, initialisms (e.g. HTTP, URL, ID, API, JSON, UUID, IP) must be fully capitalized or fully lowercase in names. Use `userID` instead of `userId`, `httpServer` instead of `HttpServer`.
- **Nesting**: Indent early and return early. Handle error cases and special conditions first.
- **Global Variables**: Unexported package-level variables and constants must be prefixed with `_` (except unexported error variables starting with `err`).
- **Struct Initialization**: Always use field names (except in test tables with 3 or fewer fields). Use `var` for zero-valued structs. Use `&T{}` instead of `new(T)`.
- **Table Tests**: Use `tests` slice and `tt` loops. Redeclare loop variables when using `t.Parallel()`. Keep tests simple; do not use complex mocks or conditionals inside subtests.
- **Functional Options**: Use for optional constructor arguments, especially when 3 or more options exist.
- **Avoid Using Built-In Names**: Never name variables, fields, or functions after Go built-in identifiers (such as `error`, `string`, `len`, `make`, `new`, `close`, etc.).
- **Struct Tags**: Struct fields marshaled into JSON, YAML, etc. must have explicit tag annotations (e.g., `` `json:"name"` ``).
- **Import Aliasing**: Use import aliases only when the package name doesn't match the last element of its import path, or to avoid direct naming conflicts.
- **Local Variable Declarations**: Use short declaration `:=` when setting an explicit value. Use `var` when declaring zero-value variables or empty slices.
- **Nil Slices**: `nil` is a valid slice of length 0. Return `nil` instead of empty slices like `[]int{}`. To check if empty, use `len(s) == 0`, not comparison to `nil`.
- **Avoid Naked Parameters**: Add comments `/* ... */` to document naked boolean or constant arguments in function calls, or use custom types instead of booleans for better type-safety.

---

## 6. Verification Workflow

When writing or modifying Go code:
1. Run `goimports -w <file>` on save to organize imports.
2. Run `go vet ./...` to check for correctness and variable shadowing.
3. If `golangci-lint` is installed, run `golangci-lint run` to verify compliance.
4. Ensure no unhandled errors, no raw panics, and no leaked goroutines.
