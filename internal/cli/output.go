package cli

import (
	"io"

	"github.com/rishiyaduwanshi/boiler/internal/ui"
	"github.com/spf13/pflag"
)

// ConfigureOutput applies the output preference before config loading and
// command discovery, which can fail before Cobra parses persistent flags.
// Cobra remains responsible for reporting invalid flags and argument values.
func ConfigureOutput(args []string) {
	flags := pflag.NewFlagSet("output", pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.ParseErrorsAllowlist.UnknownFlags = true
	flags.BoolVar(&ui.NoColor, "no-color", false, "")
	// Do not let help stop this preliminary scan before a trailing --no-color.
	flags.BoolP("help", "h", false, "")
	_ = flags.Parse(args)
}
