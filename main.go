package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"foxygo.at/protog/parser"
	"github.com/alecthomas/kong"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	pb "google.golang.org/protobuf/types/descriptorpb"
)

type cli struct {
	ImportPaths []string `short:"I" name:"proto_path" help:"Import path"`
	Out         string   `short:"o" help:"Output file. default: stdout"`
	Filename    string   `arg:"" optional:""`
	Format      string   `short:"f" help:"output protoset as one of json, pb" enum:"json,pb" default:"json"`

	in  io.Reader
	out io.Writer
}

func main() {
	c := &cli{}
	kong.Parse(c)
	if err := run(c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (c *cli) AfterApply() error {
	c.out = os.Stdout
	if c.Out != "-" && c.Out != "" {
		f, err := os.Create(c.Out)
		if err != nil {
			return err
		}
		c.out = f
	}

	c.in = os.Stdin
	if c.Filename != "-" && c.Filename != "" {
		f, err := os.Open(c.Filename)
		if err != nil {
			return err
		}
		c.in = f
	}
	return nil
}

func run(c *cli) error {
	protos, err := readProtosAndDeps(c)
	if err != nil {
		return err
	}

	fds, err := protosToFDS(protos)
	if err != nil {
		return err
	}

	return writeFDS(c.out, fds, c.Format)
}

func writeFDS(out io.Writer, fds *pb.FileDescriptorSet, format string) error {
	var err error
	var b []byte

	switch format {
	case "json":
		marshaler := protojson.MarshalOptions{Multiline: true}
		b, err = marshaler.Marshal(fds)
	case "pb":
		b, err = proto.Marshal(fds)
	default:
		err = fmt.Errorf("unknown format")
	}
	if err != nil {
		return err
	}
	_, err = out.Write(b)
	return err
}

func protosToFDS(protos []*parser.Proto) (*pb.FileDescriptorSet, error) {
	fds := make([]*pb.FileDescriptorProto, len(protos))
	var err error
	for i, proto := range protos {
		fds[i], err = protoToFD(proto)
		if err != nil {
			return nil, err
		}
	}
	return &pb.FileDescriptorSet{File: fds}, nil
}

func protoToFD(pp *parser.Proto) (*pb.FileDescriptorProto, error) {
	fd := &pb.FileDescriptorProto{
		Name: &pp.Filename,
	}
	for _, e := range pp.Entries {
		switch {
		case e.Syntax != "":
			if fd.Syntax != nil {
				return nil, errors.New("found second syntax entries")
			}
			fd.Syntax = &e.Syntax
		case e.Package != "":
			if fd.Package != nil {
				return nil, errors.New("found second Package entries")
			}
			fd.Package = &e.Package
		case e.Import != "":
			fd.Dependency = append(fd.Dependency, e.Import)
		case e.Message != nil:
			m, err := message(e.Message)
			if err != nil {
				return nil, err
			}
			fd.MessageType = append(fd.MessageType, m)
		case e.Service != nil:
		case e.Enum != nil:
		case e.Option != nil:
		case e.Extend != nil:
		default:
			return nil, errors.New("cannot interpret Entry")
		}
	}

	return fd, nil
}

func message(pm *parser.Message) (*pb.DescriptorProto, error) {
	dp := &pb.DescriptorProto{
		Name: &pm.Name,
	}
	for _, e := range pm.Entries {
		switch {
		case e.Enum != nil:
		case e.Option != nil:
		case e.Message != nil:
		case e.Oneof != nil:
		case e.Extend != nil:
		case e.Reserved != nil:
		case e.Extensions != nil:
		case e.Field != nil:
			dp.Field = append(dp.Field, field(e.Field))
		default:
			return nil, errors.New("cannot interpret MessageEntry")
		}
	}

	return dp, nil
}

func field(pf *parser.Field) *pb.FieldDescriptorProto {
	df := &pb.FieldDescriptorProto{}
	label := pb.FieldDescriptorProto_LABEL_OPTIONAL
	fieldType := pb.FieldDescriptorProto_TYPE_STRING
	if pf.Direct != nil {
		df.Name = &pf.Direct.Name
		df.Number = &pf.Direct.Tag
		df.JsonName = jsonStr(pf.Direct.Name)
		df.Type = &fieldType
		df.Label = &label
	}
	return df
}

//todo very incomplete
func jsonStr(s string) *string {
	ss := strings.Split(s, "_")
	result := strings.ToLower(ss[0])
	for _, s := range ss[1:] {
		result += strings.Title(strings.ToLower(s))
	}
	return &result
}

func readProtosAndDeps(c *cli) ([]*parser.Proto, error) {
	protos, err := readProtos(c.ImportPaths, c.in, c.Filename)
	if err != nil {
		return nil, err
	}
	var result []*parser.Proto
	seen := map[string]bool{}
	for _, proto := range protos {
		if seen[proto.Filename] {
			continue
		}
		seen[proto.Filename] = true
		result = append(result, proto)
	}
	return result, nil
}

func readProtos(paths []string, in io.Reader, filename string) ([]*parser.Proto, error) {
	p, err := parser.Parse(in)
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
	return nil, fmt.Errorf("cannot find %q on import paths", filename)
}
