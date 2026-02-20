# PocketBrain Architecture Guide

This document explains the architecture of PocketBrain, the design patterns it
uses, and the Go idioms that appear throughout the codebase.

---

## 1. High-Level Overview

PocketBrain is an autonomous AI assistant that connects to messaging channels
(currently WhatsApp), runs periodic background tasks (heartbeats), provides a
terminal UI, and exposes SSH and web terminal access. At its core it is a Go
application that follows a **hexagonal (ports & adapters) architecture**.

```
 main.go                         Entry point: TUI mode or serve mode
   |
   +-- internal/app/app.go       Wiring layer: builds and connects all services
   |     |
   |     +-- internal/config     Environment-based configuration
   |     +-- internal/store      SQLite persistence (repository implementations)
   |     +-- internal/ai         AI provider adapters (Anthropic, OpenAI, etc.)
   |     +-- internal/core       Domain logic + port interfaces
   |     +-- internal/scheduler  Heartbeat cron scheduler
   |     +-- internal/channel    Messaging channel adapters (WhatsApp)
   |     +-- internal/workspace  Sandboxed file operations
   |     +-- internal/skills     Installable skill packs
   |     +-- internal/retry      Exponential backoff utility
   |     |
   |     +-- internal/tui        Terminal UI (Bubble Tea)
   |     +-- internal/ssh        SSH server (Wish)
   |     +-- internal/web        Web terminal (xterm.js + WebSocket)
   |     +-- internal/webdav     WebDAV file server
   |     +-- internal/tsnet      Optional Tailscale mesh (build-tag gated)
   |
   +-- internal/setup            First-run setup wizard
```

---

## 2. The `internal/` Convention

Every package lives under `internal/`. In Go, the `internal` directory is a
**compiler-enforced visibility boundary**: code outside this module cannot import
anything from `internal/`. This is not just a convention -- the Go toolchain
rejects the import at build time.

Why it matters:

- It lets you freely restructure packages without worrying about external
  consumers.
- It forces a clean public API at the module root (in this case there is none;
  the binary itself is the deliverable).

---

## 3. Hexagonal Architecture (Ports & Adapters)

The most important architectural decision in PocketBrain is the separation
between **domain logic** (the "core") and **infrastructure** (databases, APIs,
messaging protocols).

### 3.1 Ports: `internal/core/ports.go`

This file defines all the **port interfaces** that the core domain depends on:

```go
type MemoryRepository interface {
    Append(fact string, source *string) (bool, error)
    Delete(id int64) (bool, error)
    Update(id int64, fact string) (bool, error)
    GetAll() ([]MemoryEntry, error)
}

type ChannelAdapter interface {
    Name() string
    Start(handler MessageHandler) error
    Stop() error
    Send(userID, text string) error
}

type HeartbeatRunner interface {
    RunHeartbeatTasks(ctx context.Context) (string, error)
}
```

These interfaces define **what the core needs**, not **how it is provided**.
The core package never imports `store`, `ai`, or `whatsapp` -- it only depends
on these interface types.

### 3.2 Adapters: `internal/store/`, `internal/ai/`, `internal/channel/`

Each adapter package provides a concrete implementation of one or more port
interfaces:

| Port Interface         | Adapter                          | File                          |
|------------------------|----------------------------------|-------------------------------|
| `MemoryRepository`     | `store.MemoryRepo`               | `internal/store/memory.go`    |
| `SessionRepository`    | `store.SessionRepo`              | `internal/store/session.go`   |
| `WhitelistRepository`  | `store.WhitelistRepo`            | `internal/store/whitelist.go` |
| `OutboxRepository`     | `store.OutboxRepo`               | `internal/store/outbox.go`    |
| `HeartbeatRepository`  | `store.HeartbeatRepo`            | `internal/store/heartbeat.go` |
| `core.Provider`        | `ai.FantasyProvider`             | `internal/ai/provider.go`     |
| `core.Provider`        | `ai.StubProvider`                | `internal/ai/provider.go`     |
| `core.ChannelAdapter`  | `whatsapp.Adapter`               | `internal/channel/whatsapp/`  |

### 3.3 Why this matters

- **Testability**: You can test `AssistantCore` with a stub provider and
  in-memory repository without any SQLite or network calls.
- **Swappability**: Switching from Anthropic to OpenAI requires zero changes
  to core logic -- just wire a different `ai.FantasyProviderConfig`.
- **Dependency direction**: Dependencies always point inward. Adapters import
  `core`; `core` never imports adapters.

---

## 4. Key Go Patterns Used

### 4.1 Compile-Time Interface Satisfaction Checks

```go
// internal/ai/provider.go
var _ core.Provider = (*FantasyProvider)(nil)
var _ core.Provider = (*StubProvider)(nil)

// internal/core/assistant.go
var _ HeartbeatRunner = (*AssistantCore)(nil)
```

This is a standard Go idiom. The blank identifier `_` discards the value, and
the type assertion `(*FantasyProvider)(nil)` creates a nil pointer of that type.
If `FantasyProvider` does not implement all methods of `core.Provider`, the
program fails to compile. This catches interface drift immediately rather than
at runtime.

### 4.2 The Options Struct Pattern

```go
type AssistantCoreOptions struct {
    Provider      Provider
    SessionMgr    *SessionManager
    PromptBuilder *PromptBuilder
    MemoryRepo    MemoryRepository
    ChannelRepo   ChannelRepository
    HeartbeatRepo HeartbeatRepository
    Logger        *slog.Logger
}

func NewAssistantCore(opts AssistantCoreOptions) *AssistantCore { ... }
```

When a constructor needs more than 3-4 parameters, Go codebases use an options
struct instead of a long parameter list. Benefits:

- Named fields make call sites self-documenting.
- Adding a new optional dependency does not break existing callers (they just
  leave the new field zero-valued).
- IDE autocompletion works well with struct field names.

This pattern appears in `AssistantCoreOptions`, `HeartbeatConfig`,
`FantasyProviderConfig`, `web.Config`, `sshsrv.Config`, and others.

### 4.3 Function Types as Interfaces

```go
// internal/core/ports.go
type MessageHandler func(userID, text string) (string, error)
```

In Go, a function type is a first-class type. When an interface would have
only a single method, a function type is more idiomatic -- it lets callers
pass a plain function or closure without wrapping it in a struct.

`ChannelAdapter.Start(handler MessageHandler)` accepts any function with that
signature. The WhatsApp adapter stores it and calls it when a message arrives.

Compare the same concept using an interface:

```go
type MessageHandler interface {
    HandleMessage(userID, text string) (string, error)
}
```

The function type version is simpler and equally type-safe.

### 4.4 Error Wrapping with `%w`

```go
return nil, fmt.Errorf("ask: get main session: %w", err)
return nil, fmt.Errorf("database: %w", err)
return nil, fmt.Errorf("fantasy: generate: %w", err)
```

Every layer wraps errors with context before returning them. The `%w` verb
(introduced in Go 1.13) makes the original error available to `errors.Is()`
and `errors.As()` for programmatic inspection up the call stack.

The pattern produces error messages like:
`config: HEARTBEAT_INTERVAL_MINUTES must be >= 1`
`backend: ai provider: fantasy: create model "bad-model": ...`

Each layer adds a prefix that tells you exactly where the failure happened
without needing a stack trace.

### 4.5 Context Propagation and Timeouts

```go
func (a *AssistantCore) Ask(ctx context.Context, input AssistantInput) (string, error) {
    if _, ok := ctx.Deadline(); !ok {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
        defer cancel()
    }
    // ...
}
```

Go passes `context.Context` as the first parameter of any function that does
I/O or may be long-running. PocketBrain uses a defensive pattern: if the caller
did not set a deadline, the function adds its own 5-minute timeout. This
prevents unbounded AI calls from hanging forever.

The `defer cancel()` is critical -- it prevents context leaks even if the
function returns early.

### 4.6 Mutex-Protected State with `sync.Mutex`

```go
// internal/store/db.go
type DB struct {
    conn *sqlite3.Conn
    mu   sync.Mutex
}

func (db *DB) exec(fn func() error) error {
    db.mu.Lock()
    defer db.mu.Unlock()
    return fn()
}
```

SQLite does not support concurrent writes on a single connection. The `DB`
struct serializes all access through a mutex. The `exec` method wraps every
operation, ensuring the lock is always acquired and released correctly via
`defer`.

This same pattern appears in:
- `ai.FantasyProvider` (protecting in-memory session history)
- `whatsapp.Adapter` (protecting `stopped` flag and `handler` reference)
- `app.shutdown` (protecting the `closers` slice)
- `tui.EventBus` (protecting subscriber map with `sync.RWMutex`)

### 4.7 `sync.Once` for One-Time Operations

```go
// internal/app/shutdown.go
type shutdown struct {
    once    sync.Once
    closers []func()
    // ...
}

func (s *shutdown) run() {
    s.once.Do(func() {
        // cleanup logic runs exactly once
    })
}
```

`sync.Once` guarantees that the cleanup function runs exactly once, even if
`run()` is called from multiple goroutines (e.g., signal handler + direct
caller). This is the idiomatic way to handle one-time teardown in Go.

### 4.8 Channel-Based Signaling

```go
// internal/scheduler/heartbeat.go
type HeartbeatScheduler struct {
    stopCh chan struct{}
    // ...
}

func (s *HeartbeatScheduler) Stop() {
    select {
    case <-s.stopCh:
        // already closed
    default:
        close(s.stopCh)
    }
}
```

A `chan struct{}` is a zero-allocation signaling channel. Closing it broadcasts
to all goroutines selecting on it. The `select` guard in `Stop()` makes it
safe to call multiple times (closing an already-closed channel panics in Go).

The scheduler loop uses a three-way select:

```go
select {
case <-s.stopCh:     // explicit stop
    return
case <-ctx.Done():   // context cancellation
    return
case <-ticker.C:     // next tick
    // do work
}
```

This is the standard Go pattern for interruptible periodic work.

### 4.9 Atomic Operations for Lock-Free State

```go
running atomic.Int32

// In the tick loop:
if !s.running.CompareAndSwap(0, 1) {
    s.log.Warn("heartbeat tick skipped: previous run still active")
    continue
}
// after run completes:
s.running.Store(0)
```

`atomic.Int32` provides lock-free concurrency control. CompareAndSwap atomically
checks "is this 0?" and sets it to 1 if so, returning whether the swap
succeeded. This prevents two heartbeat runs from overlapping without needing a
mutex.

### 4.10 Named Return Values for Deferred Error Capture

```go
// internal/store/sqlite_helpers.go
func withStmt(conn *sqlite3.Conn, query string, fn func(*sqlite3.Stmt) error) (err error) {
    stmt, _, err := conn.Prepare(query)
    if err != nil {
        return err
    }
    defer func() {
        if closeErr := stmt.Close(); err == nil && closeErr != nil {
            err = closeErr
        }
    }()
    return fn(stmt)
}
```

The named return `(err error)` lets the deferred function modify the return
value. If `fn` succeeds but `stmt.Close()` fails, the deferred closure
overwrites `err` with the close error. Without the named return, the close
error would be silently lost.

### 4.11 Transactions with Deferred Rollback

```go
// internal/store/db.go
func (db *DB) withTx(fn func() error) (err error) {
    if err := db.conn.Exec("BEGIN"); err != nil {
        return err
    }
    defer func() {
        if err != nil {
            _ = db.conn.Exec("ROLLBACK")
        }
    }()
    if err = fn(); err != nil {
        return err
    }
    return db.conn.Exec("COMMIT")
}
```

The deferred rollback fires automatically if any error occurs, including panics.
The `_ =` discards the rollback error (if we are already in an error path, the
rollback error is secondary).

---

## 5. Wiring Layer: `internal/app/app.go`

The `startBackendInternal` function is the **composition root** -- the one
place where all dependencies are created and wired together:

```
config.Load()
  -> store.Open()
    -> NewMemoryRepo(), NewSessionRepo(), etc.
      -> buildAgentTools()
        -> ai.NewFantasyProvider()
          -> core.NewSessionManager()
            -> core.NewPromptBuilder()
              -> core.NewAssistantCore()
                -> scheduler.NewHeartbeatScheduler()
                  -> startWhatsApp()
```

This is **constructor-based dependency injection** without any DI framework.
Each component receives its dependencies as explicit parameters. The wiring
is plain Go code, which means:

- The compiler catches missing dependencies.
- You can trace the flow with your editor's "go to definition".
- There is no magic, no reflection, no annotations.

---

## 6. Graceful Shutdown

PocketBrain uses a LIFO (Last-In, First-Out) shutdown pattern:

```go
type shutdown struct {
    closers []func()
    once    sync.Once
    // ...
}

func (s *shutdown) addCloser(fn func()) {
    s.closers = append(s.closers, fn)
}

func (s *shutdown) run() {
    s.once.Do(func() {
        for i := len(closers) - 1; i >= 0; i-- {
            closers[i]()
        }
        s.db.Close()
    })
}
```

Components register teardown functions as they start up. During shutdown,
they execute in reverse order (LIFO), so dependent services stop before
the things they depend on. The database closes last.

The shutdown is triggered by OS signals:

```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, ...)
go func() {
    <-sigCh
    cancel() // cancel root context
}()
```

This is idiomatic Go signal handling: a buffered channel with capacity 1
(to avoid missing the signal if the goroutine is briefly busy) and
`signal.Notify` to relay OS signals into it.

---

## 7. Event Bus: Backend-to-TUI Communication

```go
type EventBus struct {
    mu          sync.RWMutex
    subscribers map[int]chan Event
}

func (b *EventBus) Publish(e Event) {
    b.mu.RLock()
    defer b.mu.RUnlock()
    for _, ch := range b.subscribers {
        select {
        case ch <- e:
        default: // drop if subscriber is slow
        }
    }
}
```

The EventBus is a fan-out pub/sub system built on Go channels. Key decisions:

- **Non-blocking publish**: The `select/default` pattern ensures a slow
  subscriber cannot block the publisher. Events are dropped rather than
  causing backpressure.
- **RWMutex**: Multiple publishers can publish concurrently (read lock);
  only subscribe/unsubscribe needs an exclusive write lock.
- **Buffered channels**: Each subscriber gets a buffered channel (default 512)
  to absorb bursts.

---

## 8. The AI Provider Layer

### 8.1 The Provider Interface

```go
type Provider interface {
    SendMessage(ctx context.Context, sessionID, system, userText string) (string, error)
    SendMessageNoReply(ctx context.Context, sessionID, userText string) error
    CreateSession(ctx context.Context, title string) (string, error)
    RecentContext(ctx context.Context, sessionID string) (string, error)
}
```

This is the boundary between PocketBrain's domain logic and the LLM. The
core never knows whether it is talking to Anthropic, OpenAI, Google, or a
local model.

### 8.2 Fantasy Abstraction

`FantasyProvider` uses the `charm.land/fantasy` library which itself
abstracts across multiple LLM providers. The provider selection is a
simple switch:

```go
switch cfg.ProviderName {
case "anthropic": provider, err = anthropic.New(...)
case "google":    provider, err = google.New(...)
case "openai":    provider, err = openai.New(...)
default:          provider, err = openaicompat.New(...)
}
```

### 8.3 StubProvider

When no API key is configured, the wiring layer falls back to `StubProvider`
which returns a canned message. This lets the app start and be interacted with
during development without requiring API credentials.

---

## 9. The Repository Pattern

Every data access type follows the same structure:

```go
// Port (in core/ports.go)
type MemoryRepository interface { ... }

// Adapter (in store/memory.go)
type MemoryRepo struct {
    db *DB
}

func NewMemoryRepo(db *DB) *MemoryRepo {
    return &MemoryRepo{db: db}
}

func (r *MemoryRepo) Append(fact string, source *string) (bool, error) {
    // all access goes through r.db.exec(func() error { ... })
}
```

The `db.exec()` wrapper serializes all operations through the mutex. Each
repository method follows the pattern:

1. Acquire lock via `db.exec`.
2. Prepare a statement with `withStmt`.
3. Bind parameters, step through results.
4. Return Go types (not SQL types).

The `withStmt` helper (in `sqlite_helpers.go`) ensures statements are always
closed, even on error:

```go
func withStmt(conn *sqlite3.Conn, query string, fn func(*sqlite3.Stmt) error) (err error) {
    stmt, _, err := conn.Prepare(query)
    if err != nil { return err }
    defer func() {
        if closeErr := stmt.Close(); err == nil && closeErr != nil {
            err = closeErr
        }
    }()
    return fn(stmt)
}
```

---

## 10. Security-Conscious Workspace

The `workspace` package enforces path security to prevent directory traversal:

1. **Relative path resolution**: All user-provided paths are joined with the
   root and then checked with `filepath.Rel` to ensure the result does not
   escape (`..` prefix check).
2. **Symlink segment scanning**: Every path segment is checked with `os.Lstat`
   to detect symlinks that could redirect writes outside the root.
3. **Post-mkdir safety check**: After creating directories, the target is
   re-verified to prevent TOCTOU (time-of-check-time-of-use) races.
4. **EvalSymlinks on both sides**: The resolved root and target are both
   run through `filepath.EvalSymlinks` to compare real paths.

This defense-in-depth approach is important because the workspace is exposed
to the AI model via tool calls, so it must be resilient to adversarial paths.

---

## 11. Build Tags for Optional Features

```go
// internal/tsnet/stub.go (default build)
//go:build !tsnet

func Available() bool { return false }

// internal/tsnet/listener.go (tsnet build)
//go:build tsnet

func Available() bool { return true }
```

Go build tags conditionally include files at compile time. PocketBrain uses
this for Tailscale integration: by default the tsnet code is stubbed out
(zero binary size impact), and users opt in with `go build -tags tsnet`.

---

## 12. Embedded Static Assets

```go
//go:embed static
var staticFS embed.FS
```

The `embed` package (Go 1.16+) bakes the `static/` directory into the binary
at compile time. The web terminal serves its HTML/JS/CSS from this embedded
filesystem, which means the binary is fully self-contained with no external
file dependencies.

---

## 13. Structured Logging with `log/slog`

PocketBrain uses Go's standard `log/slog` package (Go 1.21+) throughout:

```go
logger.Info("assistant request started",
    "operationID", opID,
    "channel", input.Channel,
    "userID", input.UserID,
    "sessionID", sessionID,
    "textLength", len(input.Text),
)
```

In headless mode, logs are JSON (machine-parseable). In TUI mode, a custom
`BusHandler` publishes log records to the EventBus so they appear in the
TUI's log panel. The handler implements the `slog.Handler` interface:

```go
type BusHandler struct {
    bus   *tui.EventBus
    level slog.Level
    attrs []slog.Attr
}

func (h *BusHandler) Handle(_ context.Context, r slog.Record) error {
    // convert to LogEvent and publish to bus
}
```

This is a clean example of the adapter pattern applied to logging.

---

## 14. Testing Patterns

The codebase uses several testing techniques:

- **Interface-based mocking**: Because the core depends on interfaces, tests
  provide stub implementations (e.g., `StubProvider`).
- **`newFantasyProviderWithModel`**: An unexported constructor that accepts a
  mock `fantasy.LanguageModel`, enabling unit tests for the provider without
  real API calls.
- **Table-driven tests**: Following the standard Go convention of
  `[]struct{ name string; ... }` test cases.
- **`WAClient` interface**: The WhatsApp adapter depends on a `WAClient`
  interface, not the concrete `whatsmeow` client, so tests can substitute
  a mock.

---

## 15. Summary of Key Go Idioms

| Idiom                              | Where                              | Why                                        |
|------------------------------------|------------------------------------|--------------------------------------------|
| `internal/` package boundary       | Entire codebase                    | Compiler-enforced encapsulation             |
| `var _ Interface = (*Type)(nil)`   | `ai/provider.go`, `core/assistant` | Compile-time interface check                |
| Options struct constructors        | `AssistantCoreOptions`, `Config`   | Clean multi-param constructors              |
| Function types as interfaces       | `MessageHandler`                   | Simpler than single-method interfaces       |
| Error wrapping with `%w`           | Every layer                        | Preserves error chain for `errors.Is/As`    |
| `context.Context` first parameter  | All I/O functions                  | Cancellation and deadline propagation       |
| `defer cancel()` after `WithTimeout` | `Ask()`, `RunHeartbeatTasks()`   | Prevents context/goroutine leaks            |
| `sync.Mutex` + `defer Unlock()`   | `DB.exec()`, provider history      | Safe concurrent access                      |
| `sync.Once`                        | `shutdown.run()`                   | Exactly-once execution guarantee            |
| `chan struct{}` for signaling       | `HeartbeatScheduler.stopCh`        | Zero-allocation broadcast via `close()`     |
| `select/default` non-blocking send | `EventBus.Publish()`               | Drop events rather than block publishers    |
| `atomic.Int32.CompareAndSwap`      | Scheduler overlap guard            | Lock-free mutual exclusion                  |
| Named returns for deferred capture | `withStmt()`, `withTx()`           | Deferred cleanup can modify the return err  |
| `//go:embed`                       | `web/server.go`                    | Self-contained binary with static assets    |
| `//go:build` tags                  | `tsnet/`                           | Optional features without binary bloat      |
| `log/slog` structured logging      | Entire codebase                    | Typed, machine-parseable log output         |
