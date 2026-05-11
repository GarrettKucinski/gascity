package mail

import (
	"regexp"
	"strings"
	"sync"
)

// DefaultEscalationKeywords is the built-in list of subject substrings that
// mark a message as an escalation. Operators may override or extend this list
// via city.toml [mail].escalation_keywords. The list is the single source of
// truth for wake-on-escalation behavior: agents configured with
// wake_on_escalation = true are nudged when an inbound message subject
// matches any of these keywords (case-insensitive, whole-word).
var DefaultEscalationKeywords = []string{
	"ESCALATION",
	"RECOVERY",
	"RECOVERY_NEEDED",
	"MERGE_FAILED",
	"BLOCKED",
	"STUCK",
	"DIVERGENCE",
	"DIVERGED",
	"PAUSE",
}

var (
	escalationRegexCacheMu sync.Mutex
	escalationRegexCache   = map[string]*regexp.Regexp{}
)

// IsEscalationSubject reports whether subject matches any escalation keyword
// from keywords using a case-insensitive whole-word match. When keywords is
// empty, DefaultEscalationKeywords is used. Empty subject never matches.
//
// "Whole-word" means the keyword is bounded by non-word characters or the
// start/end of the string. This makes "ESCALATION: foo" match while
// "preescalation" or "deescalate" do not.
func IsEscalationSubject(subject string, keywords []string) bool {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return false
	}
	if len(keywords) == 0 {
		keywords = DefaultEscalationKeywords
	}
	re := escalationRegexFor(keywords)
	if re == nil {
		return false
	}
	return re.MatchString(subject)
}

// escalationRegexFor returns a cached compiled regex for the keyword set.
// Keywords are normalized (trimmed, uppercased, deduplicated) before
// compilation so different orderings or casings reuse the same regex.
func escalationRegexFor(keywords []string) *regexp.Regexp {
	seen := make(map[string]struct{}, len(keywords))
	normalized := make([]string, 0, len(keywords))
	for _, kw := range keywords {
		kw = strings.ToUpper(strings.TrimSpace(kw))
		if kw == "" {
			continue
		}
		if _, dup := seen[kw]; dup {
			continue
		}
		seen[kw] = struct{}{}
		normalized = append(normalized, kw)
	}
	if len(normalized) == 0 {
		return nil
	}
	key := strings.Join(normalized, "|")

	escalationRegexCacheMu.Lock()
	defer escalationRegexCacheMu.Unlock()
	if re, ok := escalationRegexCache[key]; ok {
		return re
	}
	escaped := make([]string, len(normalized))
	for i, kw := range normalized {
		escaped[i] = regexp.QuoteMeta(kw)
	}
	pattern := `(?i)\b(?:` + strings.Join(escaped, "|") + `)\b`
	re := regexp.MustCompile(pattern)
	escalationRegexCache[key] = re
	return re
}
