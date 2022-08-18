// This code is derived from https://github.com/alecthomas/protobuf

// Package parser contains a protobuf parser.
// nolint: govet, golint
package parser

import (
	"io"
	"math/big"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

var parser = participle.MustBuild(
	&Proto{},
	participle.UseLookahead(2),
	participle.Map(unquote, "String"),
	participle.Lexer(lex),
	participle.Elide("Whitespace", "Comment"),
)

// Parse protobuf.
func Parse(filename string, r io.Reader) (*Proto, error) {
	p := &Proto{}
	return p, parser.Parse(filename, r, p)
}

func ParseString(filename string, source string) (*Proto, error) {
	p := &Proto{}
	return p, parser.ParseString(filename, source, p)
}

type Proto struct {
	Pos lexer.Position

	Syntax  string    `( "syntax" "=" @String ";" )?`
	Package []Package `( @@` // there can be only 1, but it can be anywhere in the file
	Imports []Import  `| @@`
	Options []Option  `| "option" @@ ";"`
	Entries []Entry   `| @@`
	Empty   Empty     `| ";" )*`
}

type Package struct {
	Pos lexer.Position

	Name FullIdent `"package" @@ ";"`
}

type Import struct {
	Pos lexer.Position

	Public bool   `"import" ( @"public" )?`
	Name   string `@String ";"`
}

type Entry struct {
	Pos lexer.Position

	Message *Message `( @@`
	Enum    *Enum    `| @@`
	Extend  *Extend  `| @@`
	Service *Service `| @@ )`
}

type Message struct {
	Pos lexer.Position

	Name    string         `"message" @Ident`
	Entries []MessageEntry `"{" @@* "}"`
}

type MessageEntry struct {
	Pos lexer.Position

	Enum       *Enum       `( @@`
	Message    *Message    `| @@`
	Oneof      *OneOf      `| @@`
	Extend     *Extend     `| @@`
	Reserved   *Reserved   `| @@`
	Extensions *Extensions `| @@`
	Option     *Option     `| "option" @@ ";"`
	Field      *Field      `| @@` // must be after Option, or option parses as Field
	Empty      Empty       `| ";" )`
}

type Field struct {
	Pos lexer.Position

	Optional bool `( @"optional"`
	Required bool `| @"required"`
	Repeated bool `| @"repeated" )?`

	Group  *Group  `( @@`
	Direct *Direct `| @@ ";" )`
}

type Direct struct {
	Pos lexer.Position

	Type *Type  `@@`
	Name string `@Ident`
	Tag  int    `"=" @Int`

	Options []*Option `( "[" @@ ( "," @@ )* "]" )?`
}

type Group struct {
	Pos lexer.Position

	Name    string         `"group" @Ident`
	Tag     int            `"=" @Int`
	Options []Option       `( "[" @@ ( "," @@ )* "]" )?`
	Entries []MessageEntry `"{" @@* "}"`
}

type OneOf struct {
	Pos lexer.Position

	Name    string       `"oneof" @Ident`
	Entries []OneOfEntry `"{" @@* "}"`
}

type OneOfEntry struct {
	Pos lexer.Position

	Field  *Field  `( @@`
	Option *Option `| "option" @@ ";"`
	Empty  Empty   `| ";" )`
}

type Extend struct {
	Pos lexer.Position

	Reference FQIdent `"extend" @@`
	Fields    []Field `"{" @@* "}"`
}

type Reserved struct {
	Pos lexer.Position

	Ranges     []Range  `"reserved" ( @@ ( "," @@ )*`
	FieldNames []string `           | @String ( "," @String )* ) ";"`
}

type Extensions struct {
	Pos lexer.Position

	Extensions []Range  `"extensions" @@ ( "," @@ )*`
	Options    []Option `( "[" @@ ( "," @@ )* "]" )?`
}

type Range struct {
	Start int  `@Int`
	End   *int `  ( "to" ( @Int`
	Max   bool `         | @"max" ) )?`
}

type Option struct {
	Pos lexer.Position

	Name  []OptionName `@@ ( "." @@ )*`
	Value *Value       `"=" @@`
}

type OptionName struct {
	Pos lexer.Position

	Name      string   `@Ident`
	Extension *FQIdent `| "(" @@ ")"`
}

type Enum struct {
	Pos lexer.Position

	Name   string      `"enum" @Ident`
	Values []EnumEntry `"{" @@* "}"`
}

type EnumEntry struct {
	Pos lexer.Position

	Value    *EnumValue `( @@`
	Reserved *Reserved  `| @@`
	Option   *Option    `| "option" @@ ";"`
	Empty    Empty      `| ";" )`
}

type EnumValue struct {
	Pos lexer.Position

	Key   string `@Ident`
	Value int    `"=" @( ( "-" )? Int )`

	Options []Option `( "[" @@ ( "," @@ )* "]" )? ";"`
}

type Service struct {
	Pos lexer.Position

	Name    string         `"service" @Ident`
	Entries []ServiceEntry `"{" @@* "}"`
}

type ServiceEntry struct {
	Pos lexer.Position

	Method *Method `( @@`
	Option *Option `| "option" @@ ";"`
	Empty  Empty   `| ";" )`
}

type Method struct {
	Pos lexer.Position

	Name              string   `"rpc" @Ident`
	StreamingRequest  bool     `"(" ( @"stream" )?`
	Request           *Type    `    @@ ")"`
	StreamingResponse bool     `"returns" "(" ( @"stream" )?`
	Response          *Type    `              @@ ")"`
	Options           []Option `( ( "{" ( "option" @@ ";"+ )* "}" ) | ";")`
}

type FullIdent struct {
	Pos lexer.Position

	Parts []string `@Ident ( "." @Ident )*`
}

func (fi *FullIdent) String() string { return strings.Join(fi.Parts, ".") }

type FQIdent struct {
	Pos lexer.Position

	FullyQualified bool     `( @"." )?`
	Parts          []string `@Ident ( "." @Ident )*`
}

func NewFQIdentFromString(ident string) *FQIdent {
	parts := strings.Split(ident, ".")
	if len(parts) == 0 {
		return nil
	}
	if parts[0] == "" {
		return &FQIdent{FullyQualified: true, Parts: parts[1:]}
	}
	return &FQIdent{Parts: parts}
}

func (fqi *FQIdent) String() (result string) {
	if fqi.FullyQualified {
		result = "."
	}
	result += strings.Join(fqi.Parts, ".")
	return result
}

type Type struct {
	Pos lexer.Position

	Scalar    Scalar   `  @@`
	Map       *MapType `| @@`
	Reference *FQIdent `| @@`
}

type MapType struct {
	Pos lexer.Position

	Key   Scalar `"map" "<" @@`
	Value *Type  `"," @@ ">"`
}

type Value struct {
	Pos lexer.Position

	String    *string    `  @String+`
	Number    *big.Float `| ("-" | "+")? (@Float | @Int)`
	Bool      *Boolean   `| @("true"|"false")`
	ProtoText *ProtoText `| "{" @@ "}"`
	Array     *Array     `| @@`
	Reference *FQIdent   `| @@`
}

type Boolean bool

func (b *Boolean) Capture(v []string) error { *b = v[0] == "true"; return nil }

type ProtoText struct {
	Pos lexer.Position

	Fields []ProtoTextField `( @@ ( "," | ";" )? )*`
}

type ProtoTextField struct {
	Pos lexer.Position

	Name  string `( @Ident`
	Type  string `| "[" @("."? Ident ( ("." | "/") Ident )* ) "]" )`
	Value *Value `( ":"? @@ )`
}

type Array struct {
	Pos lexer.Position

	Elements []*Value `"[" ( @@ ( ","? @@ )* )? "]"`
}

type Empty struct{}

type Scalar int

const (
	None Scalar = iota
	Double
	Float
	Int32
	Int64
	Uint32
	Uint64
	Sint32
	Sint64
	Fixed32
	Fixed64
	SFixed32
	SFixed64
	Bool
	String
	Bytes
)

var scalarToString = map[Scalar]string{
	None: "None", Double: "Double", Float: "Float", Int32: "Int32", Int64: "Int64", Uint32: "Uint32",
	Uint64: "Uint64", Sint32: "Sint32", Sint64: "Sint64", Fixed32: "Fixed32", Fixed64: "Fixed64",
	SFixed32: "SFixed32", SFixed64: "SFixed64", Bool: "Bool", String: "String", Bytes: "Bytes",
}

func (s Scalar) GoString() string { return scalarToString[s] }

var stringToScalar = map[string]Scalar{
	"double": Double, "float": Float, "int32": Int32, "int64": Int64, "uint32": Uint32, "uint64": Uint64,
	"sint32": Sint32, "sint64": Sint64, "fixed32": Fixed32, "fixed64": Fixed64, "sfixed32": SFixed32,
	"sfixed64": SFixed64, "bool": Bool, "string": String, "bytes": Bytes,
}

func (s *Scalar) Parse(lex *lexer.PeekingLexer) error {
	token := lex.Peek()
	scalar, ok := stringToScalar[token.Value]
	if !ok {
		return participle.NextMatch
	}
	lex.Next()
	*s = scalar
	return nil
}
