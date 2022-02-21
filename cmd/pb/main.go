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
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
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
	Protoset    *registry.Files `short:"P" help:"Protoset of Message being translated" required:""`
	Out         string          `short:"o" help:"Output file name"`
	InFormat    string          `short:"I" help:"Input format (j[son], p[b],t[xt])" enum:"json,pb,txt,j,p,t," default:""`
	OutFormat   string          `short:"O" help:"Output format (j[son], p[b],t[xt])" enum:"json,pb,txt,j,p,t," default:""`
	Zero        bool            `short:"z" help:"Print zero values in JSON output"`
	MessageType string          `arg:"" help:"Message type to be translated" required:""`
	In          string          `arg:"" help:"Message value JSON encoded" optional:""`
}

func main() {
	kctx := kong.Parse(&cli,
		kong.Description(description),
		kong.Vars{"version": fmt.Sprintf("%s (%s on %s)", version, commit, date)},
		kong.TypeMapper(reflect.TypeOf(cli.PBConfig.Protoset), kong.MapperFunc(registryMapper)),
	)
	kctx.FatalIfErrorf(kctx.Run())
}

type unmarshaler func([]byte, proto.Message) error
type marshaler func(proto.Message) ([]byte, error)

func (cfg *PBConfig) Run() error {
	md, err := lookupMessage(cfg.Protoset, cfg.MessageType)
	if err != nil {
		return err
	}
	in, err := cfg.readInput()
	if err != nil {
		return err
	}
	unmarshal, err := cfg.unmarshaler()
	if err != nil {
		return fmt.Errorf("cannot decode %q input: %w", cfg.inFormat(), err)
	}
	message := dynamicpb.NewMessage(md)
	if err := unmarshal(in, message); err != nil {
		return err
	}
	marshal, err := cfg.marshaler()
	if err != nil {
		return err
	}
	b, err := marshal(message)
	if err != nil {
		return err
	}
	return cfg.writeOutput(b)
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
		o := protojson.UnmarshalOptions{Resolver: c.Protoset}
		return o.Unmarshal, nil
	case "pb":
		o := proto.UnmarshalOptions{Resolver: c.Protoset}
		return o.Unmarshal, nil
	case "txt":
		o := prototext.UnmarshalOptions{Resolver: c.Protoset}
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
			Resolver:        c.Protoset,
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
		o := prototext.MarshalOptions{Resolver: c.Protoset, Multiline: true}
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

func lookupMessage(reg *registry.Files, name string) (protoreflect.MessageDescriptor, error) {
	var result []protoreflect.MessageDescriptor
	reg.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		for i := 0; i < fd.Messages().Len(); i++ {
			md := fd.Messages().Get(i)
			mds, exactMatch := lookupMessageInMD(md, name)
			if exactMatch {
				result = mds
				return false
			}
			result = append(result, mds...)
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

func lookupMessageInMD(md protoreflect.MessageDescriptor, name string) (mds []protoreflect.MessageDescriptor, exactMatch bool) {
	mdName := string(md.FullName())
	if name == mdName || name == "."+mdName {
		// If we have a full name match, we're done and will also
		// ignore any other partial name matches.
		return []protoreflect.MessageDescriptor{md}, true
	}
	mdLowerName := "." + strings.ToLower(mdName)
	lowerName := strings.ToLower(name)
	if lowerName == mdLowerName || strings.HasSuffix(mdLowerName, "."+lowerName) {
		mds = append(mds, md)
	}
	subMessages := md.Messages()
	for i := 0; i < subMessages.Len(); i++ {
		md = subMessages.Get(i)
		subMDs, exactMatch := lookupMessageInMD(md, name)
		if exactMatch {
			return subMDs, true
		}
		mds = append(mds, subMDs...)
	}
	return mds, false
}

func registryMapper(kctx *kong.DecodeContext, target reflect.Value) error {
	reg, ok := target.Interface().(*registry.Files)
	if !ok {
		panic("target is not a *registry.Files")
	}
	var filename string
	if err := kctx.Scan.PopValueInto("file", &filename); err != nil {
		return err
	}
	files, err := registryFiles(filename)
	if err != nil {
		return err
	}
	*reg = *files
	return nil
}

func registryFiles(filename string) (*registry.Files, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	fds := descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(b, &fds); err != nil {
		return nil, err
	}
	return registry.NewFiles(&fds)
}

func isTTY() bool {
	_, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	return err == nil
}
