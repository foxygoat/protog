package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"foxygo.at/protog/registry"
	"github.com/alecthomas/kong"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	_ "google.golang.org/protobuf/types/known/anypb"
	_ "google.golang.org/protobuf/types/known/apipb"
	_ "google.golang.org/protobuf/types/known/durationpb"
	_ "google.golang.org/protobuf/types/known/emptypb"
	_ "google.golang.org/protobuf/types/known/fieldmaskpb"
	_ "google.golang.org/protobuf/types/known/sourcecontextpb"
	_ "google.golang.org/protobuf/types/known/structpb"
	_ "google.golang.org/protobuf/types/known/timestamppb"
	_ "google.golang.org/protobuf/types/known/typepb"
	_ "google.golang.org/protobuf/types/known/wrapperspb"
	_ "google.golang.org/protobuf/types/pluginpb"
)

var (
	// version vars set by goreleaser
	version = "tip"
	commit  = "HEAD"
	date    = "now"

	description = `
pb translates encoded Protobuf message from one format to another
`
	cli struct {
		PBConfig
		Version kong.VersionFlag `help:"Show version."`
	}
)

type PBConfig struct {
	Protoset *descriptorpb.FileDescriptorSet `short:"P" help:"Protoset containing Message to be translated"`

	Out         string `short:"o" help:"Output file name"`
	InFormat    string `short:"I" help:"Input format (j[son], p[b], t[xt])" enum:"json,pb,txt,j,p,t," default:""`
	OutFormat   string `short:"O" help:"Output format (j[son], p[b], t[xt])" enum:"json,pb,txt,j,p,t," default:""`
	Zero        bool   `short:"z" help:"Print zero values in JSON output"`
	MessageType string `arg:"" help:"Message type to be translated"`
	In          string `arg:"" help:"Message value JSON encoded" optional:""`

	types *protoregistry.Types
}

func main() {
	kctx := kong.Parse(&cli,
		kong.Description(description),
		kong.Vars{"version": fmt.Sprintf("%s (%s on %s)", version, commit, date)},
		kong.TypeMapper(reflect.TypeOf(cli.PBConfig.Protoset), kong.MapperFunc(fdsMapper)),
	)
	kctx.FatalIfErrorf(kctx.Run())
}

type unmarshaler func([]byte, proto.Message) error
type marshaler func(proto.Message) ([]byte, error)

func (c *PBConfig) Run() error {
	c.types = registry.CloneTypes(protoregistry.GlobalTypes)
	if c.Protoset != nil {
		if err := registry.AddDynamicTypes(c.types, c.Protoset); err != nil {
			return err
		}
	}

	mt, err := lookupMessage(c.types, c.MessageType)
	if err != nil {
		return err
	}
	in, err := c.readInput()
	if err != nil {
		return err
	}
	unmarshal, err := c.unmarshaler()
	if err != nil {
		return fmt.Errorf("cannot decode %q input: %w", c.inFormat(), err)
	}
	message := mt.New().Interface()
	if err := unmarshal(in, message); err != nil {
		return err
	}
	if fds, ok := message.(*descriptorpb.FileDescriptorSet); ok {
		if err := registry.AddDynamicTypes(c.types, fds); err != nil {
			return err
		}
		// Unmarshal again with the input in the resolver registry so
		// that any exensions defined and used in the input are
		// unmarshaled properly.
		if err := unmarshal(in, message); err != nil {
			return err
		}
	}
	marshal, err := c.marshaler()
	if err != nil {
		return err
	}
	b, err := marshal(message)
	if err != nil {
		return err
	}
	return c.writeOutput(b)
}

func (c *PBConfig) AfterApply() error {
	if c.Zero && c.outFormat() != "json" {
		return fmt.Errorf(`cannot print zero values with %q, only "json"`, c.outFormat())
	}
	return nil
}

func (c *PBConfig) readInput() ([]byte, error) {
	if c.In == "" {
		return io.ReadAll(os.Stdin)
	}
	if strings.HasPrefix(c.In, "@") {
		return os.ReadFile(c.In[1:])
	}
	return []byte(c.In), nil
}

func (c *PBConfig) writeOutput(b []byte) error {
	if c.Out == "" {
		if getFormat("", c.OutFormat) == "pb" && isTTY() {
			return fmt.Errorf("not writing binary to terminal. Use -O json or -O txt to output a textual format")
		}
		_, err := os.Stdout.Write(b)
		return err
	}
	return os.WriteFile(c.Out, b, 0666)
}

func (c *PBConfig) unmarshaler() (unmarshaler, error) {
	switch c.inFormat() {
	case "json":
		o := protojson.UnmarshalOptions{Resolver: c.types}
		return o.Unmarshal, nil
	case "pb":
		o := proto.UnmarshalOptions{Resolver: c.types}
		return o.Unmarshal, nil
	case "txt":
		o := prototext.UnmarshalOptions{Resolver: c.types}
		return o.Unmarshal, nil
	}
	return nil, fmt.Errorf("unknown input format %q", c.inFormat())
}

func (c *PBConfig) inFormat() string {
	return getFormat(c.In, c.InFormat)
}

func (c *PBConfig) outFormat() string {
	return getFormat("@"+c.Out, c.OutFormat)
}

func (c *PBConfig) marshaler() (marshaler, error) {
	switch c.outFormat() {
	case "json":
		o := protojson.MarshalOptions{
			Resolver:        c.types,
			Multiline:       true,
			EmitUnpopulated: c.Zero,
		}
		return func(m proto.Message) ([]byte, error) {
			b, err := o.Marshal(m)
			if err != nil {
				return nil, err
			}
			return append(b, byte('\n')), nil
		}, nil
	case "pb":
		o := proto.MarshalOptions{}
		return o.Marshal, nil
	case "txt":
		o := prototext.MarshalOptions{Resolver: c.types, Multiline: true}
		return o.Marshal, nil
	}
	return nil, fmt.Errorf("unknown output format %s", c.outFormat())
}

func getFormat(contentOrFile string, format string) string {
	if format != "" {
		return canonicalFormat(format)
	}
	ext := filepath.Ext(contentOrFile)
	// default to JSON for stdout, string input and files without extension
	if contentOrFile == "@" || !strings.HasPrefix(contentOrFile, "@") || ext == "" {
		return "json"
	}
	return canonicalFormat(ext[1:])
}

func canonicalFormat(format string) string {
	switch format {
	case "json", "j":
		return "json"
	case "pb", "p":
		return "pb"
	case "txt", "t", "prototext", "prototxt":
		return "txt"
	}
	return format
}

func lookupMessage(types *protoregistry.Types, name string) (protoreflect.MessageType, error) {
	var result []protoreflect.MessageType
	types.RangeMessages(func(mt protoreflect.MessageType) bool {
		mdName := string(mt.Descriptor().FullName())
		if name == mdName || name == "."+mdName {
			// If we have a full name match, we're done and will also
			// ignore any other partial name matches.
			result = []protoreflect.MessageType{mt}
			return false
		}
		mdLowerName := "." + strings.ToLower(mdName)
		lowerName := strings.ToLower(name)
		if lowerName == mdLowerName || strings.HasSuffix(mdLowerName, "."+lowerName) {
			result = append(result, mt)
		}
		return true
	})

	if len(result) == 0 {
		return nil, fmt.Errorf("message not found: %s", name)
	}
	if len(result) > 1 {
		return nil, fmt.Errorf("ambiguous message name: %s", name)
	}
	return result[0], nil
}

func fdsMapper(kctx *kong.DecodeContext, target reflect.Value) error {
	fds, ok := target.Interface().(*descriptorpb.FileDescriptorSet)
	if !ok {
		panic("target is not a *descriptorpb.FileDescriptorSet")
	}
	var filename string
	if err := kctx.Scan.PopValueInto("file", &filename); err != nil {
		return err
	}
	b, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return proto.Unmarshal(b, fds)
}

func isTTY() bool {
	_, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	return err == nil
}
