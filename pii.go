package tape

import "regexp"

// PIIPattern is a named regex pattern for PII detection.
type PIIPattern struct {
	Name        string
	Regex       string
	Replacement string // what to replace matches with, e.g. "[SSN]"
	compiled    *regexp.Regexp
}

// PIIRedactor scans for personally identifiable information in the stream
// and replaces matches with placeholder text via TransformAction.
type PIIRedactor struct {
	patterns []PIIPattern
}

// NewPIIRedactor creates a redactor from regex patterns.
// Invalid patterns are silently skipped.
func NewPIIRedactor(patterns []PIIPattern) *PIIRedactor {
	compiled := make([]PIIPattern, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p.Regex)
		if err != nil {
			continue
		}
		replacement := p.Replacement
		if replacement == "" {
			replacement = "[REDACTED]"
		}
		compiled = append(compiled, PIIPattern{
			Name:        p.Name,
			Regex:       p.Regex,
			Replacement: replacement,
			compiled:    re,
		})
	}
	return &PIIRedactor{patterns: compiled}
}

func (m *PIIRedactor) Name() string { return "pii_redactor" }

func (m *PIIRedactor) Scan(buf []byte, prevTail string) MatchResult {
	var scanTarget []byte
	if prevTail != "" {
		scanTarget = append([]byte(prevTail), buf...)
	} else {
		scanTarget = buf
	}

	for _, p := range m.patterns {
		if p.compiled.Match(scanTarget) {
			return FullMatch
		}
	}
	return NoMatch
}

// Extract replaces all PII matches in the buffer and returns a TransformAction.
func (m *PIIRedactor) Extract(buf []byte) FilterAction {
	result := make([]byte, len(buf))
	copy(result, buf)

	for _, p := range m.patterns {
		result = p.compiled.ReplaceAll(result, []byte(p.Replacement))
	}
	return TransformAction{Text: string(result)}
}
