@AGENTS.md

## Conventions
- StreamFilter maintains a lookahead buffer; tokens emit from the trailing edge with delay
- Matchers implement Scan() returning NoMatch, PartialMatch, or FullMatch
- Extractable matchers can produce text transformations; non-extractable matchers can only kill the stream
- Cross-boundary matching uses prevTail overlap (20 chars)

## Constraints
- Stdlib only; no external dependencies
- Do not import any axon-* packages
- Matchers must be stateless between Write() calls; all state lives in the StreamFilter buffer

## Testing
- `go test ./...` runs all tests
- `go vet ./...` for lint
