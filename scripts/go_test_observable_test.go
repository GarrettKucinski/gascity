package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// stubGoEmittingJSONL writes a /bin/sh stub at binDir/go that ignores its
// arguments, prints the supplied go-test -json JSONL stream on stdout, and
// exits with exitCode. Used to drive scripts/go-test-observable through
// controlled failure scenarios without invoking the real Go toolchain.
func stubGoEmittingJSONL(t *testing.T, binDir, jsonl string, exitCode int) {
	t.Helper()
	stub := "#!/bin/sh\ncat <<'JSONL'\n" + jsonl + "JSONL\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(filepath.Join(binDir, "go"), []byte(stub), 0o755); err != nil {
		t.Fatalf("write go stub: %v", err)
	}
}

func runGoTestObservable(t *testing.T, repoRoot, binDir string, args ...string) (string, error) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "observable.jsonl")
	cmd := exec.Command(filepath.Join(repoRoot, "scripts", "go-test-observable"), args...)
	cmd.Env = []string{
		"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"OBSERVABLE_TEST_LOG=" + logPath,
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestGoTestObservableEmitsFailedTestNames is a regression guard for ga-szlnd.
// The failure-detail path previously used bash 4 `mapfile -t`, which exited
// with status 127 on macOS's stock bash 3.2.57 and masked the failure list.
// The portable while-read replacement must surface each failed test by name.
func TestGoTestObservableEmitsFailedTestNames(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skipf("jq required for failure-detail path: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	binDir := t.TempDir()
	jsonl := `{"Time":"2026-05-12T12:00:00Z","Action":"run","Test":"TestAlpha","Package":"p"}
{"Time":"2026-05-12T12:00:01Z","Action":"output","Test":"TestAlpha","Output":"--- FAIL: TestAlpha\n","Package":"p"}
{"Time":"2026-05-12T12:00:01Z","Action":"fail","Test":"TestAlpha","Package":"p","Elapsed":0.01}
{"Time":"2026-05-12T12:00:00Z","Action":"run","Test":"TestBeta","Package":"p"}
{"Time":"2026-05-12T12:00:02Z","Action":"output","Test":"TestBeta","Output":"--- FAIL: TestBeta\n","Package":"p"}
{"Time":"2026-05-12T12:00:02Z","Action":"fail","Test":"TestBeta","Package":"p","Elapsed":0.02}
`
	stubGoEmittingJSONL(t, binDir, jsonl, 1)

	out, err := runGoTestObservable(t, repoRoot, binDir, "stub-fail", "--", "./...")
	if err == nil {
		t.Fatalf("expected non-zero exit (stub go returned 1); got success\noutput:\n%s", out)
	}
	for _, want := range []string{"failed test: TestAlpha", "failed test: TestBeta"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\noutput:\n%s", want, out)
		}
	}
}

// TestGoTestObservableHandlesPackageOnlyFailure covers the empty-failed-tests
// branch. If go test emits only a package-level fail (no per-test fail events),
// print_failure_details must fall through to the "no test-level failure event"
// notice and still print the output tail. This branch also exercises the
// explicit `local -a failed_tests=()` declaration that keeps `set -u` happy
// when no fail events are present.
func TestGoTestObservableHandlesPackageOnlyFailure(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skipf("jq required for failure-detail path: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	binDir := t.TempDir()
	jsonl := `{"Time":"2026-05-12T12:00:00Z","Action":"output","Package":"p","Output":"compile error: undefined symbol\n"}
{"Time":"2026-05-12T12:00:00Z","Action":"fail","Package":"p","Elapsed":0.0}
`
	stubGoEmittingJSONL(t, binDir, jsonl, 2)

	out, err := runGoTestObservable(t, repoRoot, binDir, "stub-pkg-fail", "--", "./...")
	if err == nil {
		t.Fatalf("expected non-zero exit (stub go returned 2); got success\noutput:\n%s", out)
	}
	if !strings.Contains(out, "no test-level failure event was emitted") {
		t.Errorf("output missing package-only-failure notice\noutput:\n%s", out)
	}
}
