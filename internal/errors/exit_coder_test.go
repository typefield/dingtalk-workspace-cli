package errors

import (
	stderrors "errors"
	"strings"
	"testing"
)

type stubExitCoder struct{ code int }

func (s *stubExitCoder) Error() string { return "stub" }
func (s *stubExitCoder) ExitCode() int { return s.code }

type stubRawStderr struct{ raw string }

func (s *stubRawStderr) Error() string     { return s.raw }
func (s *stubRawStderr) RawStderr() string { return s.raw }

func TestExitCode_ExitCoderInterface(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"exit code 4 via interface", &stubExitCoder{code: 4}, 4},
		{"exit code 1 via interface", &stubExitCoder{code: 1}, 1},
		{"framework Error takes precedence", NewAPI("api"), 1},
		{"plain error falls back to 5", stderrors.New("plain"), 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExitCode(tc.err); got != tc.want {
				t.Errorf("ExitCode() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestExitCode_WrappedExitCoder(t *testing.T) {
	t.Parallel()
	wrapped := stderrors.Join(stderrors.New("context"), &stubExitCoder{code: 4})
	if got := ExitCode(wrapped); got != 4 {
		t.Errorf("ExitCode(wrapped) = %d, want 4", got)
	}
}

func TestRawStderrError_Interface(t *testing.T) {
	t.Parallel()
	err := &stubRawStderr{raw: `{"code":"PAT_LOW_RISK_NO_PERMISSION"}`}
	var raw RawStderrError
	if !stderrors.As(err, &raw) {
		t.Fatal("expected errors.As to match RawStderrError")
	}
	if !strings.Contains(raw.RawStderr(), "PAT_LOW_RISK_NO_PERMISSION") {
		t.Errorf("RawStderr() = %q, want PAT code", raw.RawStderr())
	}
}
