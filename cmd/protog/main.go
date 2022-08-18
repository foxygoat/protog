package main

import (
	"fmt"
	"os"

	"foxygo.at/protog/compiler/parser"
	"github.com/alecthomas/kong"
)

var (
	// version vars set by goreleaser
	version = "tip"
	commit  = "HEAD"
	date    = "now"

	description = `protog compiles .proto files`

	cli struct {
		ProtogConfig
		Version kong.VersionFlag `help:"Show version"`
	}
)

type ProtogConfig struct {
	Filename []string `arg:"" help:"filenames of .proto file to compile"`
}

func main() {
	kctx := kong.Parse(&cli,
		kong.Description(description),
		kong.Vars{"version": fmt.Sprintf("%s (%s on %s)", version, commit, date)},
	)
	if err := kctx.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (cfg *ProtogConfig) Run() error {
	for _, filename := range cfg.Filename {
		f, err := os.Open(filename)
		if err != nil {
			return err
		}
		p, err := parser.Parse(filename, f)
		if err != nil {
			return err
		}
		fmt.Printf("%+v\n", p)
	}
	return nil
}
