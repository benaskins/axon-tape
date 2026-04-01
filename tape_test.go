package tape

import (
	"fmt"
	"strings"
	"testing"
)

// collector gathers emitted text for assertions.
type collector struct {
	chunks []string
}

func (c *collector) emit(s string) {
	c.chunks = append(c.chunks, s)
}

func (c *collector) all() string {
	return strings.Join(c.chunks, "")
}

// --- StreamFilter buffer mechanics ---

func TestStreamFilter_PassthroughSmallTokens(t *testing.T) {
	c := &collector{}
	f := NewStreamFilter(c.emit, nil, 200)

	f.Write("Hello ")
	f.Write("world")
	f.Flush()

	if c.all() != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", c.all())
	}
}

func TestStreamFilter_BufferDelaysEmission(t *testing.T) {
	c := &collector{}
	f := NewStreamFilter(c.emit, nil, 20)

	f.Write("abcdefghijklmnopqrstuvwxyz") // 26 chars, maxBuffer=20

	if len(c.chunks) == 0 {
		t.Fatal("expected emission when buffer exceeds max")
	}
	emitted := c.all()
	if len(emitted) != 6 {
		t.Errorf("expected 6 chars emitted, got %d: %q", len(emitted), emitted)
	}
	if emitted != "abcdef" {
		t.Errorf("expected 'abcdef', got %q", emitted)
	}

	f.Flush()
	if c.all() != "abcdefghijklmnopqrstuvwxyz" {
		t.Errorf("expected full alphabet, got %q", c.all())
	}
}

func TestStreamFilter_FlushEmptyBuffer(t *testing.T) {
	c := &collector{}
	f := NewStreamFilter(c.emit, nil, 200)

	action := f.Flush()
	if _, ok := action.(ContinueAction); !ok {
		t.Errorf("expected ContinueAction for empty flush, got %T", action)
	}
	if len(c.chunks) != 0 {
		t.Errorf("expected no emissions, got %d", len(c.chunks))
	}
}

func TestStreamFilter_MultipleSmallWrites(t *testing.T) {
	c := &collector{}
	f := NewStreamFilter(c.emit, nil, 50)

	for i := 0; i < 20; i++ {
		f.Write(fmt.Sprintf("tok%d ", i))
	}
	f.Flush()

	expected := ""
	for i := 0; i < 20; i++ {
		expected += fmt.Sprintf("tok%d ", i)
	}
	if c.all() != expected {
		t.Errorf("expected all tokens concatenated, got %q", c.all())
	}
}

func TestStreamFilter_PrevTailOverlap(t *testing.T) {
	c := &collector{}
	f := NewStreamFilter(c.emit, nil, 10)

	f.Write("0123456789ABCDEFGHIJ") // 20 chars, maxBuffer=10
	if f.PrevTail() == "" {
		t.Fatal("expected prevTail to be set after emission")
	}
	if len(f.PrevTail()) > defaultOverlap {
		t.Errorf("prevTail too long: %d", len(f.PrevTail()))
	}
}

// --- TransformAction handling ---

// testTransformMatcher always matches and transforms content.
type testTransformMatcher struct {
	replacement string
}

func (m *testTransformMatcher) Name() string { return "test_transform" }

func (m *testTransformMatcher) Scan(buf []byte, _ string) MatchResult {
	if strings.Contains(string(buf), "REPLACE_ME") {
		return FullMatch
	}
	return NoMatch
}

func (m *testTransformMatcher) Extract(buf []byte) FilterAction {
	replaced := strings.ReplaceAll(string(buf), "REPLACE_ME", m.replacement)
	return TransformAction{Text: replaced}
}

func TestStreamFilter_TransformAction_HandledInternally(t *testing.T) {
	c := &collector{}
	m := &testTransformMatcher{replacement: "REPLACED"}
	f := NewStreamFilter(c.emit, []Matcher{m}, 200)

	action := f.Write("before REPLACE_ME after")
	// Consumer should see ContinueAction, not TransformAction
	if _, ok := action.(ContinueAction); !ok {
		t.Fatalf("expected ContinueAction, got %T", action)
	}

	// Transformed text should have been emitted via emitFunc
	if !strings.Contains(c.all(), "REPLACED") {
		t.Errorf("expected transformed text emitted, got %q", c.all())
	}
	if strings.Contains(c.all(), "REPLACE_ME") {
		t.Error("original text should not appear in output")
	}
}

func TestStreamFilter_TransformAction_OnFlush(t *testing.T) {
	c := &collector{}
	m := &testTransformMatcher{replacement: "DONE"}
	f := NewStreamFilter(c.emit, []Matcher{m}, 500)

	// Write in small pieces that won't trigger during Write
	f.Write("before ")
	f.Write("REPLACE_ME")
	f.Write(" after")
	action := f.Flush()

	if _, ok := action.(ContinueAction); !ok {
		t.Fatalf("expected ContinueAction from flush, got %T", action)
	}
	if !strings.Contains(c.all(), "DONE") {
		t.Errorf("expected transformed text, got %q", c.all())
	}
}

// --- ContentSafetyMatcher ---

func TestContentSafetyMatcher_DirectMatch(t *testing.T) {
	patterns := []ContentSafetyPattern{
		{Name: "test_block", Regex: `blocked_word`},
	}
	m := NewContentSafetyMatcher(patterns)

	if r := m.Scan([]byte("This contains blocked_word here"), ""); r != FullMatch {
		t.Errorf("expected FullMatch, got %v", r)
	}
}

func TestContentSafetyMatcher_CaseInsensitive(t *testing.T) {
	patterns := []ContentSafetyPattern{
		{Name: "test_block", Regex: `blocked_word`},
	}
	m := NewContentSafetyMatcher(patterns)

	if r := m.Scan([]byte("This contains BLOCKED_WORD here"), ""); r != FullMatch {
		t.Errorf("expected FullMatch (case insensitive), got %v", r)
	}
}

func TestContentSafetyMatcher_NoMatch(t *testing.T) {
	patterns := []ContentSafetyPattern{
		{Name: "test_block", Regex: `blocked_word`},
	}
	m := NewContentSafetyMatcher(patterns)

	if r := m.Scan([]byte("This is perfectly safe text"), ""); r != NoMatch {
		t.Errorf("expected NoMatch, got %v", r)
	}
}

func TestContentSafetyMatcher_WithOverlap(t *testing.T) {
	patterns := []ContentSafetyPattern{
		{Name: "test_block", Regex: `blocked`},
	}
	m := NewContentSafetyMatcher(patterns)

	if r := m.Scan([]byte("ked content here"), "bloc"); r != FullMatch {
		t.Errorf("expected FullMatch with overlap, got %v", r)
	}
}

func TestContentSafetyMatcher_InvalidRegex(t *testing.T) {
	patterns := []ContentSafetyPattern{
		{Name: "bad", Regex: `[invalid`},
		{Name: "good", Regex: `blocked`},
	}
	m := NewContentSafetyMatcher(patterns)

	if r := m.Scan([]byte("this is blocked"), ""); r != FullMatch {
		t.Errorf("expected FullMatch from valid pattern, got %v", r)
	}
	if len(m.patterns) != 1 {
		t.Errorf("expected 1 compiled pattern, got %d", len(m.patterns))
	}
}

func TestStreamFilter_ContentSafetyKill(t *testing.T) {
	patterns := []ContentSafetyPattern{
		{Name: "blocked", Regex: `forbidden_phrase`},
	}
	safety := NewContentSafetyMatcher(patterns)
	c := &collector{}
	f := NewStreamFilter(c.emit, []Matcher{safety}, 200)

	action := f.Write("This contains a forbidden_phrase that should be blocked")

	if _, ok := action.(KillAction); !ok {
		t.Fatalf("expected KillAction, got %T", action)
	}
	if c.all() != "" {
		t.Errorf("expected no emission on kill, got %q", c.all())
	}
}
