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
		b = append(b, '\n')
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
		case e.Enum != nil:
			e, err := enum(e.Enum)
			if err != nil {
				return nil, err
			}
			fd.EnumType = append(fd.EnumType, e)
		case e.Service != nil:
		case e.Option != nil:
		case e.Extend != nil:
		default:
			return nil, errors.New("cannot interpret Entry")
		}
	}

	return fd, nil
}

func enum(pe *parser.Enum) (*pb.EnumDescriptorProto, error) {
	e := &pb.EnumDescriptorProto{
		Name: &pe.Name,
	}
	for _, pev := range pe.Values {
		switch {
		case pev.Value != nil:
			ev, err := enumValue(pev.Value)
			if err != nil {
				return nil, err
			}
			e.Value = append(e.Value, ev)
		case pev.Option != nil: // TODO
		case pev.Reserved != nil:
			reservedRanges, reservedNames, err := reserved(pev.Reserved)
			if err != nil {
				return nil, err
			}
			e.ReservedRange = append(e.ReservedRange, reservedRanges...)
			e.ReservedName = append(e.ReservedName, reservedNames...)
		default:
			return nil, errors.New("cannot interpret EnumEntry")
		}
	}
	return e, nil
}

func reserved(pr *parser.Reserved) ([]*pb.EnumDescriptorProto_EnumReservedRange, []string, error) {
	var reservedRanges []*pb.EnumDescriptorProto_EnumReservedRange
	var reservedNames []string
	for _, r := range pr.Reserved {
		if r.Ident != "" {
			reservedNames = append(reservedNames, r.Ident)
			continue
		}
		start := r.Start
		er := &pb.EnumDescriptorProto_EnumReservedRange{
			Start: &start,
			End:   &start,
		}
		if r.End != nil {
			end := *r.End
			er.End = &end
		}
		if r.Max {
			var end int32 = 2147483647 // tested with protoc 🤷‍♀️
			er.End = &end
		}
		reservedRanges = append(reservedRanges, er)
	}
	return reservedRanges, reservedNames, nil
}

func enumValue(pev *parser.EnumValue) (*pb.EnumValueDescriptorProto, error) {
	e := &pb.EnumValueDescriptorProto{
		Name:    &pev.Key,
		Number:  &pev.Value,
		Options: nil, // TODO
	}
	return e, nil
}

func message(pm *parser.Message) (*pb.DescriptorProto, error) {
	dp := &pb.DescriptorProto{
		Name: &pm.Name,
	}
	for _, e := range pm.Entries {
		switch {
		case e.Enum != nil:
			et, err := enum(e.Enum)
			if err != nil {
				return nil, err
			}
			dp.EnumType = append(dp.EnumType, et)
		case e.Option != nil:
		case e.Message != nil:
		case e.Oneof != nil:
		case e.Extend != nil:
		case e.Reserved != nil:
		case e.Extensions != nil:
		case e.Field != nil:
			df, err := field(e.Field)
			if err != nil {
				return nil, err
			}
			dp.Field = append(dp.Field, df)
		default:
			return nil, errors.New("cannot interpret MessageEntry")
		}
	}

	return dp, nil
}

var scalars = map[parser.Scalar]pb.FieldDescriptorProto_Type{
	parser.Double:   pb.FieldDescriptorProto_TYPE_DOUBLE,
	parser.Float:    pb.FieldDescriptorProto_TYPE_FLOAT,
	parser.Int32:    pb.FieldDescriptorProto_TYPE_INT32,
	parser.Int64:    pb.FieldDescriptorProto_TYPE_INT64,
	parser.Uint32:   pb.FieldDescriptorProto_TYPE_UINT32,
	parser.Uint64:   pb.FieldDescriptorProto_TYPE_UINT64,
	parser.Sint32:   pb.FieldDescriptorProto_TYPE_SINT32,
	parser.Sint64:   pb.FieldDescriptorProto_TYPE_SINT64,
	parser.Fixed32:  pb.FieldDescriptorProto_TYPE_FIXED32,
	parser.Fixed64:  pb.FieldDescriptorProto_TYPE_FIXED64,
	parser.SFixed32: pb.FieldDescriptorProto_TYPE_SFIXED32,
	parser.SFixed64: pb.FieldDescriptorProto_TYPE_SFIXED64,
	parser.Bool:     pb.FieldDescriptorProto_TYPE_BOOL,
	parser.String:   pb.FieldDescriptorProto_TYPE_STRING,
	parser.Bytes:    pb.FieldDescriptorProto_TYPE_BYTES,
}

func field(pf *parser.Field) (*pb.FieldDescriptorProto, error) {
	df := &pb.FieldDescriptorProto{}
	label := pb.FieldDescriptorProto_LABEL_OPTIONAL

	if pf.Direct == nil {
		return nil, errors.New("non-direct not implemented")
	}
	if pf.Direct.Type.Scalar == parser.None {
		return nil, errors.New("non-scalar not implemented")
	}

	fieldType, ok := scalars[pf.Direct.Type.Scalar]
	// ignoring maps and reference right now
	if !ok {
		return nil, fmt.Errorf("unknown scalar type: %d", pf.Direct.Type.Scalar)
	}

	df.Name = &pf.Direct.Name
	df.Number = &pf.Direct.Tag
	df.JsonName = jsonStr(pf.Direct.Name)
	df.Type = &fieldType
	df.Label = &label

	return df, nil
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
