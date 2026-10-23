package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/ArchL-Systems/docforge/internal/build"
)

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, err := fmt.Fprintln(stdout, "Usage: docforge build [--source docs] [--output dist]")
		return err
	}

	if args[0] != "build" {
		return fmt.Errorf("unknown command %q; use docforge --help", args[0])
	}

	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	source := flags.String("source", "docs", "directory containing Markdown files")
	output := flags.String("output", "dist", "directory for the generated site")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}

		return err
	}

	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}

	count, err := build.Run(*source, *output)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(stdout, "Generated %d pages in %s\n", count, *output)
	return err
}
