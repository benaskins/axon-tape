// Package tape provides a buffered token stream filter with pluggable matchers.
//
// A StreamFilter sits between a token source and an output sink. Tokens feed
// into a lookahead buffer; matchers scan the buffer for patterns. Text emits
// from the trailing edge with a fixed delay, giving matchers time to detect
// patterns that span multiple tokens.
//
// Think of it as a sampler sitting in front of the play head of a tape machine
// — it can inspect, transform, or suppress content before it plays.
package tape

import "bytes"

// --- Filter actions ---

// FilterAction is returned by StreamFilter.Write and Flush to signal what happened.
// The filter handles ContinueAction and TransformAction internally. Only actions
// that require a consumer decision (KillAction, domain-specific actions) surface.
type FilterAction interface{ filterAction() }

// ContinueAction signals that nothing requires consumer attention.
type ContinueAction struct{}

// TransformAction signals that matched content was replaced.
// The filter emits Text via emitFunc and returns ContinueAction to the consumer.
type TransformAction struct{ Text string }

// KillAction signals that the stream should be terminated.
type KillAction struct{ Reason string }

func (ContinueAction) filterAction()   {}
func (TransformAction) filterAction()  {}
func (KillAction) filterAction()       {}

// --- Matcher interface ---

// MatchResult is the outcome of a matcher scanning the buffer.
type MatchResult int

const (
	NoMatch      MatchResult = iota
	PartialMatch             // keep buffering, pattern might be forming
	FullMatch                // pattern confirmed, act on it
)

// Matcher scans a buffer and reports whether a pattern is present.
// prevTail contains the last N chars of previously emitted text for cross-boundary matching.
type Matcher interface {
	Scan(buf []byte, prevTail string) MatchResult
	Name() string
}

// Extractable is implemented by matchers that produce data or transformations on FullMatch.
type Extractable interface {
	Extract(buf []byte) FilterAction
}

// --- StreamFilter ---

const (
	DefaultMaxBuffer = 200
	defaultOverlap   = 20
)

// StreamFilter sits between a token source and an output sink.
// It maintains a small lookahead buffer, runs matchers against it, and
// emits tokens from the trailing edge with a fixed delay.
type StreamFilter struct {
	buf       bytes.Buffer
	emitFunc  func(string)
	matchers  []Matcher
	maxBuffer int
	overlap   int
	prevTail  string
}

// NewStreamFilter creates a filter with the given emit function, matchers, and buffer size.
func NewStreamFilter(emitFunc func(string), matchers []Matcher, maxBuffer int) *StreamFilter {
	if maxBuffer <= 0 {
		maxBuffer = DefaultMaxBuffer
	}
	return &StreamFilter{
		emitFunc:  emitFunc,
		matchers:  matchers,
		maxBuffer: maxBuffer,
		overlap:   defaultOverlap,
	}
}

// Write feeds a token into the filter. Returns an action only when the consumer
// needs to make a decision (e.g. KillAction). Text emission and transformations
// are handled internally via emitFunc.
func (f *StreamFilter) Write(token string) FilterAction {
	f.buf.WriteString(token)
	return f.scan()
}

// Flush drains the remaining buffer. Call after the token source completes.
func (f *StreamFilter) Flush() FilterAction {
	if f.buf.Len() == 0 {
		return ContinueAction{}
	}

	bufBytes := f.buf.Bytes()
	for _, m := range f.matchers {
		result := m.Scan(bufBytes, f.prevTail)
		if result == FullMatch {
			if ext, ok := m.(Extractable); ok {
				action := ext.Extract(bufBytes)
				f.buf.Reset()
				return f.handle(action)
			}
			f.buf.Reset()
			return KillAction{Reason: m.Name()}
		}
	}

	f.emit(f.buf.String())
	f.buf.Reset()
	return ContinueAction{}
}

// PrevTail returns the overlap text for external inspection.
func (f *StreamFilter) PrevTail() string {
	return f.prevTail
}

// scan runs matchers and emits/holds buffer content accordingly.
func (f *StreamFilter) scan() FilterAction {
	bufBytes := f.buf.Bytes()

	for _, m := range f.matchers {
		result := m.Scan(bufBytes, f.prevTail)
		switch result {
		case FullMatch:
			if ext, ok := m.(Extractable); ok {
				action := ext.Extract(bufBytes)
				f.buf.Reset()
				return f.handle(action)
			}
			f.buf.Reset()
			return KillAction{Reason: m.Name()}

		case PartialMatch:
			return ContinueAction{}
		}
	}

	// No matches — emit overflow beyond maxBuffer
	if f.buf.Len() > f.maxBuffer {
		emitN := f.buf.Len() - f.maxBuffer
		toEmit := string(bufBytes[:emitN])
		f.emit(toEmit)

		remaining := make([]byte, f.maxBuffer)
		copy(remaining, bufBytes[emitN:])
		f.buf.Reset()
		f.buf.Write(remaining)
	}

	return ContinueAction{}
}

// handle processes an action from a matcher. TransformAction is handled
// internally by emitting the transformed text. Everything else passes through.
func (f *StreamFilter) handle(action FilterAction) FilterAction {
	if t, ok := action.(TransformAction); ok {
		f.emit(t.Text)
		return ContinueAction{}
	}
	return action
}

// emit sends text to the sink and saves the tail for overlap matching.
func (f *StreamFilter) emit(s string) {
	if s == "" {
		return
	}
	f.emitFunc(s)

	if len(s) >= f.overlap {
		f.prevTail = s[len(s)-f.overlap:]
	} else {
		combined := f.prevTail + s
		if len(combined) > f.overlap {
			f.prevTail = combined[len(combined)-f.overlap:]
		} else {
			f.prevTail = combined
		}
	}
}
