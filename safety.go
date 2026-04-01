package tape

import "regexp"

// ContentSafetyPattern is a named regex pattern for content filtering.
type ContentSafetyPattern struct {
	Name     string `json:"name"`
	Regex    string `json:"regex"`
	compiled *regexp.Regexp
}

// ContentSafetyMatcher scans for blocked patterns in the stream.
// On match, it produces a KillAction to terminate the stream.
type ContentSafetyMatcher struct {
	patterns []ContentSafetyPattern
}

// NewContentSafetyMatcher creates a matcher from regex patterns.
// Invalid patterns are silently skipped.
func NewContentSafetyMatcher(patterns []ContentSafetyPattern) *ContentSafetyMatcher {
	compiled := make([]ContentSafetyPattern, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile("(?i)" + p.Regex)
		if err != nil {
			continue
		}
		compiled = append(compiled, ContentSafetyPattern{
			Name:     p.Name,
			Regex:    p.Regex,
			compiled: re,
		})
	}
	return &ContentSafetyMatcher{patterns: compiled}
}

func (m *ContentSafetyMatcher) Name() string { return "content_safety" }

func (m *ContentSafetyMatcher) Scan(buf []byte, prevTail string) MatchResult {
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
