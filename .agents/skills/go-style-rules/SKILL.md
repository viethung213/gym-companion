---
name: go-style-rules
description: "Rules and conventions for writing and modifying Go code. Use this skill EVERY TIME you are about to write, edit, refactor, or generate Go code. Do NOT load or use this skill when answering general design, architectural, or conceptual questions that do not involve writing or modifying code."
---

# Go Programming Style Guide Rules

1. **No Pointers to Interfaces**: Never use pointers to interfaces (`*MyInterface`). Pass the interface value directly.
2. **Compile-time Interface Verification**: Use anonymous global variable assignments (`var _ Interface = (*Struct)(nil)`) to verify that a struct implements an interface at compile-time.
3. **Receiver Types**: Use pointer receivers (`*T`) if methods modify state, if the struct contains a Mutex, or if the struct is large. Use value receivers (`T`) for small, immutable types with no pointers or mutexes. If any method uses a pointer receiver, all methods of that struct must use pointer receivers.
4. **Mutex Best Practices**: Mutexes have valid zero-values; do not initialize with pointers.
5. **No Mutex Embedding**: Do not embed Mutexes in public or private structs; keep them private (e.g., `mu sync.Mutex`).
6. **Copy Slices and Maps on Input**: Copy slices and maps when receiving them as function arguments if you store a reference to them, to prevent external modifications.
7. **Copy Slices and Maps on Output**: Copy slices and maps representing internal state before returning them from struct fields to prevent external modification and data races.
8. **Defer to Clean Up**: Use `defer` to clean up resources such as files, connections, and locks.
9. **Channel Buffer Limit**: Default to unbuffered (size 0) or buffer size 1 channels. Any other size must be justified with design or benchmark.
10. **Buffered Channels for OS Signals**: Use buffered channels of size 1 when receiving signals from `os/signal` to avoid missing notifications.
11. **Start Enums at One**: Standard Go enums should start at one (`iota + 1`) to distinguish them from uninitialized zero values, unless zero is the desired default.
12. **Use time.Time for Instants**: Use the `"time"` package to handle time. Use `time.Time` for instants of time and its methods for comparisons and additions.
13. **Use time.Duration for Periods**: Use `time.Duration` when dealing with periods of time.
14. **Use time.Time and time.Duration with External Systems**: Use `time.Time` (as RFC 3339 string) and `time.Duration` in interactions with external systems (JSON, database, YAML, flags) when possible. If not possible, include the unit in the field name (e.g., `IntervalMillis`).
15. **Error Types**: Use `errors.New` for static errors (as package globals) and `fmt.Errorf` for dynamic errors. Use custom error types when the caller needs to match the error.
16. **Error Wrapping**: Use `fmt.Errorf` with `%w` if consumers need to inspect the underlying cause, or `%v` to obfuscate implementation details. Avoid "failed to" prefixes in wrapped context; keep error messages lowercase and without trailing periods.
17. **Error Naming**: Exported global errors must start with `Err` (e.g., `ErrNotFound`). Unexported global errors must start with `err` (e.g., `errDatabaseClose`). Custom error struct types must end with `Error` (e.g., `ValidationError`).
18. **Handle Errors Once**: Handle each error exactly once. Either log the error and degrade/continue, OR return/wrap the error. Never do both.
19. **Safe Type Assertions**: Always use the comma-ok idiom (`value, ok := i.(Type)`) when doing type assertion from an interface to prevent panics.
20. **Don't Panic (Error Handling)**: Use `error` for predictable/handleable errors. Use `panic` only when the program enters a state where it cannot continue (bugs, invariant violations, or some program initialization cases). Do not use panic + recover as a standard error handling mechanism.
21. **Recover Only at Boundaries**: Place `recover` only at system boundaries (e.g., HTTP middleware, RPC interceptors, background workers, schedulers) to log crashes and protect the main process.
22. **Use Typed Atomics**: Use the typed atomic types provided by the standard library (`atomic.Bool`, `atomic.Int64`, `atomic.Pointer[T]`, etc. from `sync/atomic`). Always wrap atomic state in these types to prevent direct read/write access and avoid data races. Do not use external libraries like `go.uber.org/atomic`.
23. **Avoid Mutable Globals**: Do not use global variables that can modify state. Pass configuration and clients via dependency injection or struct constructors.
24. **Global Naming and Config**: Keep global variables to a minimum. If used, name them clearly (e.g., `serverDefaultPort`, `defaultTimeout`) and do not use a leading underscore `_` prefix for unexported globals.
25. **Avoid Public Embedding**: Do not embed types (anonymous fields) in public structs. Use explicit composition (named fields) to avoid exposing internal methods in the public API.
26. **Embedding in Unexported Structs**: Position embedded types at the top of unexported structs, followed by a blank line before regular fields.
27. **Avoid Using Built-In Names**: Avoid naming variables, parameters, or functions after built-in identifiers (e.g., `len`, `make`, `new`, `append`, `nil`).
28. **Avoid `init()`**: Avoid using `init()` functions. Perform all initializations via explicit constructor functions.
29. **Exit in Main**: Only call `os.Exit` or `log.Fatal` inside `main()` of package `main`, and at most once. Never call them in lower layers (libraries/packages) as they bypass deferred cleanup.
30. **Serialization Field Tags**: Structs participating in encoding/decoding (JSON, XML, etc.) must explicitly declare field tags for all fields.
31. **No Free Goroutines**: Never launch a goroutine without controlling its lifecycle or knowing how it will terminate. Use `context.Context` to propagate cancellation signals.
32. **Goroutine Coordination**: Use `sync.WaitGroup.Go()` (Go 1.25+) or `sync.WaitGroup` to wait for goroutines to exit. Use `errgroup.Group` to manage a collection of goroutines where all should cancel if any fails.
33. **Lifecycle-Managed Components**: Design components with explicit lifecycles (e.g., `Start`/`Shutdown` or `Run`/`Close` methods) rather than running background goroutines indefinitely.
34. **No Goroutines in `init()`**: Never start background goroutines inside `init()`.
35. **Use strconv over fmt**: Use `strconv.Itoa` or similar functions instead of `fmt.Sprintf` for basic type-to-string conversions.
36. **Avoid String-Byte Conversion Loops**: Avoid repeated conversion between `string` and `[]byte` in hot paths/loops to prevent allocations.
37. **Selective Preallocation**: Preallocate map/slice capacity (`make([]T, 0, cap)`) only if it is on a hot path or justified by benchmark data. Avoid premature optimization.
38. **Line Length**: Keep lines under a soft limit of 99 characters to avoid horizontal scrolling.
39. **Group Declarations**: Group related `import`, `const`, `var`, and `type` declarations using parentheses `()`. In functions, group adjacent variables even if unrelated.
40. **Import Ordering**: Group imports into standard library first, then third-party/internal module imports, separated by a blank line. Use `goimports` to format.
41. **Naming Conventions**: Package names must be lowercase, singular, and short (no underscores or MixedCaps; do not use generic names like `util`, `common`). Function names must be `MixedCaps`. Test functions can use underscores for groupings. Use package aliases only to prevent collision.
42. **Function Grouping & Ordering**: Arrange functions by call order (top-down). Group methods by receiver. Exported functions must be defined before unexported ones. Place constructor `NewXYZ()` directly after the struct definition.
43. **Reduce Nesting**: Keep the happy path left-aligned. Handle errors/special cases early and return/continue early. Avoid unnecessary `else` blocks after a `return` or `panic`.
44. **Local Variables**: Use short names in narrow scopes, descriptive names in wide scopes. Narrow down variable scope; initialize variables in `if` statements when possible. Avoid shadowing variables.
45. **Nil vs Empty Slice**: Prefer declaring nil slices (`var s []int`) over empty slice literals (`s := []int{}`). Use empty slice literals or `make` only when JSON serialization explicitly requires a non-null empty array `[]`.
46. **Avoid Naked Parameters**: Use inline comments for ambiguous boolean or numeric function arguments (e.g., `/* isHeaderVisible = */ true`).
47. **Raw String Literals**: Use raw string literals (backticks \`) for multi-line strings or regex expressions to avoid complex escape characters.
48. **Use Field Names to Initialize Structs**: Use field names when initializing structs (except simple stdlib types).
49. **Omit Zero Value Fields in Structs**: Omit zero-value fields in struct literals.
50. **Use `var` for Zero Value Structs**: Use `var s MyStruct` for zero-value declarations instead of `s := MyStruct{}`.
51. **Initializing Struct References**: Use `&MyStruct{}` instead of `new(MyStruct)`.
52. **Initializing Maps**: Use `make(map[K]V)` or literals for maps. Use map literals when defining a fixed set of items, and `make` otherwise.
53. **Format Strings outside Printf**: Declare format strings used outside Printf functions as constants (`const`) so static analysis tools can verify them.
54. **Printf-style Naming**: Any custom function accepting format strings must end with the suffix `f` (e.g., `Errorf`, `Infof`).
55. **General Style Rules**: Ensure code passes `gofmt`, `go vet`, and `staticcheck`.
56. **Table-Driven Tests**: Use table-driven testing with `t.Run`. Use name `tests` for the slice and `tt` for the loop variable. Use fields `give` and `want` for input and output. Keep test tables simple (no complex conditional setups).
57. **Parallel Test Loop Variable**: For parallel tests (`t.Parallel()`), redefine the loop variable inside the loop (`tt := tt`).
58. **Got before Want**: Output actual value `got` before expected `want` in test errors.
59. **Use `t.Helper()`**: Use `t.Helper()` in test helper functions.
60. **Avoid Complex Assertion Libraries**: Use standard `if` checks or custom diff libraries (e.g., `go-cmp`) instead of heavy assertion frameworks.
61. **Functional Options Pattern**: Implement the Functional Options pattern for complex constructors. Define an `Option` interface with a private method `apply(*options)` to apply configuration on a private `options` struct.
