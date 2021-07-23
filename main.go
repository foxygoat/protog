package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"foxygo.at/protog/parser"
	"github.com/alecthomas/kong"
)

type CLI struct {
	ImportPaths []string `short:"I" name:"proto_path" help:"Import path"`
	Filename    string   `arg:"" optional:""`
}

func main() {
	c := &CLI{}
	kong.Parse(c)
	if err := run(c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(c *CLI) error {
	var input io.Reader = os.Stdin
	if c.Filename != "" && c.Filename != "-" {
		f, err := os.Open(c.Filename)
		if err != nil {
			return err
		}
		defer f.Close()
		input = f
	}
	protos, err := readProtos(c.ImportPaths, input, c.Filename)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, proto := range protos {
		if seen[proto.Filename] {
			continue
		}
		seen[proto.Filename] = true
		fmt.Println(proto.Filename)
	}
	return nil
}

func readProtos(paths []string, input io.Reader, filename string) ([]*parser.Proto, error) {
	p, err := parser.Parse(input)
	if err != nil {
		return nil, err
	}

	result := []*parser.Proto{}
	for _, e := range p.Entries {
		if e.Import == "" {
			continue
		}
		f, err := search(paths, e.Import)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		s, err := readProtos(paths, f, e.Import)
		if err != nil {
			return nil, err
		}
		f.Close()
		result = append(result, s...)
	}

	p.Filename = filename
	result = append(result, p)
	return result, nil
}

func search(paths []string, filename string) (io.ReadCloser, error) {
	for _, pth := range paths {
		fname := filepath.Join(pth, filename)
		if f, err := os.Open(fname); err == nil {
			return f, nil
		}
	}
	return nil, errors.New("NOOOOOO")
}
