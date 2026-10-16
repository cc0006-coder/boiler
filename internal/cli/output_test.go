package cli

import (
	"os"
	"testing"

	"github.com/rishiyaduwanshi/boiler/internal/ui"
)

func TestConfigureOutputBeforeCommandDiscovery(t *testing.T) {
	old := ui.NoColor
	oldNoColor, hadNoColor := os.LookupEnv("NO_COLOR")
	t.Cleanup(func() {
		ui.NoColor = old
		if hadNoColor {
			_ = os.Setenv("NO_COLOR", oldNoColor)
		} else {
			_ = os.Unsetenv("NO_COLOR")
		}
	})
	for _, tc := range []struct {
		name string
		args []string
		want bool
	}{
		{name: "default"},
		{name: "unknown command", args: []string{"--no-color", "missing"}, want: true},
		{name: "unknown flag", args: []string{"ls", "--invalid", "--no-color"}, want: true},
		{name: "explicit true", args: []string{"ls", "--no-color=true"}, want: true},
		{name: "explicit false", args: []string{"--no-color", "--no-color=false"}},
		{name: "help", args: []string{"--help", "--no-color"}, want: true},
		{name: "short help", args: []string{"-h", "--no-color"}, want: true},
		{name: "positional argument", args: []string{"ls", "--", "--no-color"}},
		{name: "assignment value", args: []string{"var", "example=--no-color"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ConfigureOutput(tc.args)
			if ui.NoColor != tc.want {
				t.Fatalf("NoColor = %v, want %v", ui.NoColor, tc.want)
			}
		})
	}
}

func TestConfigureOutputPropagatesNoColorToChildEnvironment(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	old := ui.NoColor
	ui.NoColor = false
	t.Cleanup(func() { ui.NoColor = old })

	ConfigureOutput([]string{"--no-color"})

	if got := os.Getenv("NO_COLOR"); got != "1" {
		t.Fatalf("NO_COLOR = %q, want %q so script children inherit --no-color", got, "1")
	}
}
