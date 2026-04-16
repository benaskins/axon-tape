# axon-tape

Buffered token stream filter with pluggable matchers.

Import: `github.com/benaskins/axon-tape`

## What it does

axon-tape sits between a token source (e.g. LLM output) and an output sink, maintaining a small lookahead buffer. Pluggable matchers scan the buffer for patterns and can transform content, redact PII, or kill the stream entirely.

Like a sampler in front of a tape machine play head: tokens pass through with a short delay while matchers inspect them.

No external dependencies (stdlib only).

## Usage

```go
import tape "github.com/benaskins/axon-tape"

filter := tape.NewStreamFilter(
    func(s string) { fmt.Print(s) },  // emit callback
    []tape.Matcher{redactor, safety},
    tape.DefaultMaxBuffer,
)

filter.Write("token")
filter.Flush()
```

## Built-in matchers

| Matcher | Action |
|---------|--------|
| `PIIRedactor` | Detects PII via regex patterns, replaces with placeholders |
| `ContentSafetyMatcher` | Blocks streams matching content safety patterns |

## Custom matchers

Implement the `Matcher` interface:

```go
type Matcher interface {
    Scan(buf []byte, prevTail string) MatchResult
    Name() string
}
```

Optionally implement `Extractable` to produce text transformations on match.

## Build & Test

```bash
go test ./...
go vet ./...
```
