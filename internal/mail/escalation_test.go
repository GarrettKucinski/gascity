package mail

import "testing"

func TestIsEscalationSubject_DefaultKeywords(t *testing.T) {
	cases := []struct {
		name    string
		subject string
		want    bool
	}{
		{"empty subject", "", false},
		{"whitespace subject", "   ", false},
		{"plain match start", "ESCALATION: refinery diverged", true},
		{"case insensitive", "escalation: refinery diverged", true},
		{"mixed case", "Escalation Needed", true},
		{"merge_failed match", "MERGE_FAILED: rebase conflict", true},
		{"recovery_needed match", "RECOVERY_NEEDED: ga-c4t", true},
		{"recovery alone", "RECOVERY: ga-c4t", true},
		{"blocked uppercase", "BLOCKED: need credentials", true},
		{"blocked lowercase", "blocked: need credentials", true},
		{"stuck word", "STUCK: 30 min on rebase", true},
		{"diverged word", "branch diverged from main", true},
		{"divergence word", "DIVERGENCE detected on origin/main", true},
		{"pause word", "PAUSE: merge freeze active", true},
		{"work done no match", "WORK_DONE: ga-xyz merged", false},
		{"fyi no match", "FYI: rotated keys", false},
		{"re thread chatter no match", "RE: status update", false},
		{"preescalation no whole-word match", "preescalation review", false},
		{"deescalate no whole-word match", "deescalate this issue", false},
		{"underscore is word boundary", "MERGE_FAILED", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsEscalationSubject(tc.subject, nil); got != tc.want {
				t.Errorf("IsEscalationSubject(%q, default) = %v, want %v", tc.subject, got, tc.want)
			}
		})
	}
}

func TestIsEscalationSubject_CustomKeywords(t *testing.T) {
	kw := []string{"PRODUCTION_DOWN", "P0", "FIRE"}
	cases := []struct {
		subject string
		want    bool
	}{
		{"PRODUCTION_DOWN: db unreachable", true},
		{"p0: pager firing", true},
		{"FIRE: rollback now", true},
		{"ESCALATION: not configured", false},
		{"production downstream check", false},
	}
	for _, tc := range cases {
		t.Run(tc.subject, func(t *testing.T) {
			if got := IsEscalationSubject(tc.subject, kw); got != tc.want {
				t.Errorf("IsEscalationSubject(%q, %v) = %v, want %v", tc.subject, kw, got, tc.want)
			}
		})
	}
}

func TestIsEscalationSubject_EmptyKeywordsFallsBackToDefault(t *testing.T) {
	if !IsEscalationSubject("ESCALATION: test", []string{}) {
		t.Error("empty keyword list should fall back to defaults")
	}
}

func TestIsEscalationSubject_NormalizesAndDeduplicates(t *testing.T) {
	// Mixed case and duplicates normalize to the same compiled regex.
	if !IsEscalationSubject("ESCALATION: test", []string{"escalation", "ESCALATION", " ESCALATION "}) {
		t.Error("normalization should treat case/whitespace/duplicates uniformly")
	}
}

func TestIsEscalationSubject_AllBlankKeywordsReturnsFalse(t *testing.T) {
	if IsEscalationSubject("ESCALATION: test", []string{"", "   "}) {
		t.Error("all-blank keyword list should match nothing")
	}
}
