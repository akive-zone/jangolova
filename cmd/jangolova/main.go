package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"jangolova/internal/manifest"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "validate":
		if err := validateCommand(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "jangolova: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "jangolova: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func validateCommand(args []string) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	path := flags.String("file", "-", "session manifest path, or - for stdin")
	flags.StringVar(path, "f", "-", "session manifest path, or - for stdin")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("validate accepts flags only")
	}

	reader := io.Reader(os.Stdin)
	var file *os.File
	if *path != "-" {
		var err error
		file, err = os.Open(*path)
		if err != nil {
			return fmt.Errorf("open manifest: %w", err)
		}
		defer file.Close()
		reader = file
	}

	value, err := manifest.Decode(reader)
	if err != nil {
		return err
	}
	fmt.Printf(
		"valid session %q: engine=%s surfaces=%d controllers=%d connectors=%d\n",
		value.Metadata.Name,
		value.Spec.Engine.Adapter,
		len(value.Spec.Surfaces),
		len(value.Spec.Controllers),
		len(value.Spec.Connectors),
	)
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage: jangolova <command>

Commands:
  validate --file PATH   Validate a v1alpha1 JSON session manifest`)
}
