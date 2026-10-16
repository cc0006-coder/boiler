package ui

import (
	"bufio"
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/creack/pty"
)

func TestStylesPreservePlainOutput(t *testing.T) {
	styles := map[string]Style{
		"error": Error, "success": Success, "info": Info, "warning": Warning,
		"path": Path, "header": Header, "muted": Muted,
	}
	for name, style := range styles {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			style.Fprintf(&out, "\n  %s: %d%%\n\n", "hello", 100)
			if got, want := out.String(), "\n  hello: 100%\n\n"; got != want {
				t.Fatalf("output = %q, want %q", got, want)
			}
		})
	}
}

func TestColorOptOut(t *testing.T) {
	old := NoColor
	t.Cleanup(func() { NoColor = old })
	for _, tc := range []struct {
		name, noColor, term string
		flag                bool
	}{
		{name: "flag", term: "xterm-256color", flag: true},
		{name: "environment", noColor: "1", term: "xterm-256color"},
		{name: "environment zero is nonempty", noColor: "0", term: "xterm-256color"},
		{name: "dumb terminal", term: "dumb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			NoColor = tc.flag
			t.Setenv("NO_COLOR", tc.noColor)
			t.Setenv("TERM", tc.term)
			t.Setenv("CLICOLOR_FORCE", "1")
			if colorEnabled(os.Stdout) {
				t.Fatal("color enabled despite explicit opt-out")
			}
			if got := Header.Render("Title"); got != "Title" {
				t.Fatalf("disabled header = %q", got)
			}
		})
	}
}

func TestRedirectedOutputIgnoresForcedColor(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("CLICOLOR_FORCE", "1")
	file, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if colorEnabled(file) {
		t.Fatal("regular files must not receive ANSI sequences")
	}
	Success.Fprintf(file, "saved %s\n", "example.go")
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := out.ReadFrom(file); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "saved example.go\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestStylesRenderColorOnTTY(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR", "1")
	t.Setenv("CLICOLOR_FORCE", "1")
	oldNoColor := NoColor
	NoColor = false
	t.Cleanup(func() { NoColor = oldNoColor })

	master, slave, err := pty.Open()
	if err != nil {
		t.Fatalf("open PTY: %v", err)
	}
	t.Cleanup(func() { _ = master.Close() })
	t.Cleanup(func() { _ = slave.Close() })

	// Prevent the terminal driver from translating LF to CRLF, so the test
	// checks the exact newlines written by the renderer.
	state, err := term.MakeRaw(slave.Fd())
	if err != nil {
		t.Fatalf("make PTY raw: %v", err)
	}
	t.Cleanup(func() { _ = term.Restore(slave.Fd(), state) })
	if !colorEnabled(slave) {
		t.Fatal("color must be enabled for the PTY slave")
	}

	// Render uses stdout rather than the writer passed to Fprintf.
	oldStdout := os.Stdout
	os.Stdout = slave
	t.Cleanup(func() { os.Stdout = oldStdout })
	path := Path.Render("example.go")
	if !strings.Contains(path, "\x1b[") {
		t.Fatalf("rendered path has no ANSI sequences: %q", path)
	}
	if got := ansi.Strip(path); got != "example.go" {
		t.Fatalf("plain rendered path = %q, want %q", got, "example.go")
	}

	type readResult struct {
		text string
		err  error
	}
	output := make(chan readResult, 1)
	go func() {
		text, err := bufio.NewReader(master).ReadString('\x00')
		output <- readResult{text: text, err: err}
	}()

	Success.Fprintf(slave, "\n\n  saved %s\n\n", path)
	// Delimit the output without relying on platform-specific PTY EOF behavior.
	if _, err := slave.Write([]byte{0}); err != nil {
		t.Fatalf("write output delimiter: %v", err)
	}
	select {
	case result := <-output:
		if result.err != nil {
			t.Fatalf("read PTY output: %v", result.err)
		}
		got := strings.TrimSuffix(result.text, "\x00")
		if !strings.Contains(got, "\x1b[") {
			t.Fatalf("colored output has no ANSI sequences: %q", got)
		}
		if !strings.HasPrefix(got, "\n\n") || !strings.HasSuffix(got, "\n\n") {
			t.Fatalf("leading/trailing newlines were not preserved: %q", got)
		}
		if plain, want := ansi.Strip(got), "\n\n  saved example.go\n\n"; plain != want {
			t.Fatalf("plain nested output = %q, want %q (colored: %q)", plain, want, got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out reading PTY output")
	}
}
