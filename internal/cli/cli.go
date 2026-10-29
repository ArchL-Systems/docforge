package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ArchL-Systems/docforge/internal/build"
)

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, err := fmt.Fprintln(stdout, "Usage: docforge build [--output dist]")
		return err
	}

	if args[0] != "build" {
		return fmt.Errorf("unknown command %q; use docforge --help", args[0])
	}

	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(stderr)
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

	root, err := os.Getwd()
	if err != nil {
		return err
	}

	count, err := build.Run(root, *output)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(stdout, "Generated %d pages in %s\n", count, *output)
	return err
}
