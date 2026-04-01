package tape

import (
	"strings"
	"testing"
)

var defaultPIIPatterns = []PIIPattern{
	{Name: "ssn", Regex: `\b\d{3}-\d{2}-\d{4}\b`, Replacement: "[SSN]"},
	{Name: "credit_card", Regex: `\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`, Replacement: "[CARD]"},
	{Name: "email", Regex: `\b[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}\b`, Replacement: "[EMAIL]"},
}

func TestPIIRedactor_SSN(t *testing.T) {
	m := NewPIIRedactor(defaultPIIPatterns)

	if r := m.Scan([]byte("My SSN is 123-45-6789"), ""); r != FullMatch {
		t.Fatalf("expected FullMatch, got %v", r)
	}

	action := m.Extract([]byte("My SSN is 123-45-6789"))
	ta, ok := action.(TransformAction)
	if !ok {
		t.Fatalf("expected TransformAction, got %T", action)
	}
	if ta.Text != "My SSN is [SSN]" {
		t.Errorf("expected 'My SSN is [SSN]', got %q", ta.Text)
	}
}

func TestPIIRedactor_CreditCard(t *testing.T) {
	m := NewPIIRedactor(defaultPIIPatterns)

	buf := []byte("Card: 4111-1111-1111-1111 on file")
	if r := m.Scan(buf, ""); r != FullMatch {
		t.Fatalf("expected FullMatch, got %v", r)
	}

	action := m.Extract(buf)
	ta := action.(TransformAction)
	if !strings.Contains(ta.Text, "[CARD]") {
		t.Errorf("expected [CARD] replacement, got %q", ta.Text)
	}
	if strings.Contains(ta.Text, "4111") {
		t.Error("card number should be redacted")
	}
}

func TestPIIRedactor_Email(t *testing.T) {
	m := NewPIIRedactor(defaultPIIPatterns)

	buf := []byte("Contact alice@example.com for details")
	if r := m.Scan(buf, ""); r != FullMatch {
		t.Fatalf("expected FullMatch, got %v", r)
	}

	action := m.Extract(buf)
	ta := action.(TransformAction)
	if ta.Text != "Contact [EMAIL] for details" {
		t.Errorf("expected 'Contact [EMAIL] for details', got %q", ta.Text)
	}
}

func TestPIIRedactor_MultiplePII(t *testing.T) {
	m := NewPIIRedactor(defaultPIIPatterns)

	buf := []byte("SSN: 123-45-6789, email: bob@test.org")
	action := m.Extract(buf)
	ta := action.(TransformAction)

	if strings.Contains(ta.Text, "123-45") {
		t.Error("SSN should be redacted")
	}
	if strings.Contains(ta.Text, "bob@") {
		t.Error("email should be redacted")
	}
	if !strings.Contains(ta.Text, "[SSN]") || !strings.Contains(ta.Text, "[EMAIL]") {
		t.Errorf("expected both replacements, got %q", ta.Text)
	}
}

func TestPIIRedactor_NoMatch(t *testing.T) {
	m := NewPIIRedactor(defaultPIIPatterns)

	if r := m.Scan([]byte("This is safe text with no PII"), ""); r != NoMatch {
		t.Errorf("expected NoMatch, got %v", r)
	}
}

func TestPIIRedactor_DefaultReplacement(t *testing.T) {
	m := NewPIIRedactor([]PIIPattern{
		{Name: "phone", Regex: `\b\d{3}-\d{3}-\d{4}\b`}, // no Replacement set
	})

	buf := []byte("Call 555-123-4567")
	action := m.Extract(buf)
	ta := action.(TransformAction)
	if !strings.Contains(ta.Text, "[REDACTED]") {
		t.Errorf("expected default [REDACTED], got %q", ta.Text)
	}
}

func TestPIIRedactor_InvalidRegex(t *testing.T) {
	m := NewPIIRedactor([]PIIPattern{
		{Name: "bad", Regex: `[invalid`},
		{Name: "ssn", Regex: `\b\d{3}-\d{2}-\d{4}\b`, Replacement: "[SSN]"},
	})

	if len(m.patterns) != 1 {
		t.Errorf("expected 1 pattern (bad skipped), got %d", len(m.patterns))
	}
}

func TestPIIRedactor_WithOverlap(t *testing.T) {
	m := NewPIIRedactor(defaultPIIPatterns)

	// SSN spans prevTail and buffer
	if r := m.Scan([]byte("45-6789 rest"), "SSN: 123-"); r != FullMatch {
		t.Errorf("expected FullMatch with overlap, got %v", r)
	}
}

// --- Integration: StreamFilter + PIIRedactor ---

func TestStreamFilter_PIIRedaction(t *testing.T) {
	c := &collector{}
	pii := NewPIIRedactor(defaultPIIPatterns)
	f := NewStreamFilter(c.emit, []Matcher{pii}, 200)

	action := f.Write("Please contact alice@example.com for more info")

	// Consumer should see ContinueAction — transform handled internally
	if _, ok := action.(ContinueAction); !ok {
		t.Fatalf("expected ContinueAction, got %T", action)
	}

	if !strings.Contains(c.all(), "[EMAIL]") {
		t.Errorf("expected email redacted, got %q", c.all())
	}
	if strings.Contains(c.all(), "alice@") {
		t.Error("email should not appear in output")
	}
}
