// This code is derived from https://github.com/alecthomas/protobuf

package parser

import (
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/alecthomas/repr"
	"github.com/stretchr/testify/require"
)

func TestParserFromFixtures(t *testing.T) {
	files, err := filepath.Glob("../testdata/*.proto")
	require.NoError(t, err)
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			r, err := os.Open(file)
			require.NoError(t, err)
			_, err = Parse(file, r)
			require.NoError(t, err)
		})
	}
}

func TestParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Proto
	}{{
		name: "MessageOptions",
		input: `
			message VariousComplexOptions {
			  option (complex_opt2).bar.(protobuf_unittest.corge).qux = 2008;
			  option (complex_opt2).(protobuf_unittest.garply).(corge).qux = 2121;
			  option (.ComplexOptionType2.ComplexOptionType4.complex_opt4).waldo = 1971;
			  option (complex_opt2).foo.bar.(baz).qux = 1980;
			  option (strings) = "1" "2";
			  option deprecated = true;
			  option deprecated = false;
			}
			`,
		expected: &Proto{
			Entries: []Entry{
				{Message: &Message{
					Name: "VariousComplexOptions",
					Entries: []MessageEntry{
						{Option: &Option{
							Name: []OptionName{
								{Extension: NewFQIdentFromString("complex_opt2")},
								{Name: "bar"},
								{Extension: NewFQIdentFromString("protobuf_unittest.corge")},
								{Name: "qux"},
							},
							Value: &Value{Number: toBig(2008)},
						}},
						{Option: &Option{
							Name: []OptionName{
								{Extension: NewFQIdentFromString("complex_opt2")},
								{Extension: NewFQIdentFromString("protobuf_unittest.garply")},
								{Extension: NewFQIdentFromString("corge")},
								{Name: "qux"},
							},
							Value: &Value{Number: toBig(2121)},
						}},
						{Option: &Option{
							Name: []OptionName{
								{Extension: NewFQIdentFromString(".ComplexOptionType2.ComplexOptionType4.complex_opt4")},
								{Name: "waldo"},
							},
							Value: &Value{Number: toBig(1971)},
						}},
						{Option: &Option{
							Name: []OptionName{
								{Extension: NewFQIdentFromString("complex_opt2")},
								{Name: "foo"},
								{Name: "bar"},
								{Extension: NewFQIdentFromString("baz")},
								{Name: "qux"},
							},
							Value: &Value{Number: toBig(1980)},
						}},
						{Option: &Option{
							Name:  []OptionName{{Extension: NewFQIdentFromString("strings")}},
							Value: &Value{String: ptr[string]("12")},
						}},
						{Option: &Option{
							Name:  []OptionName{{Name: "deprecated"}},
							Value: &Value{Bool: ptr[Boolean](true)},
						}},
						{Option: &Option{
							Name:  []OptionName{{Name: "deprecated"}},
							Value: &Value{Bool: ptr[Boolean](false)},
						}},
					},
				}},
			},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := ParseString(test.name, test.input)
			require.NoError(t, err)
			clearPos(actual)
			expectedStr := repr.String(test.expected, repr.Indent("  "))
			actualStr := repr.String(actual, repr.Indent("  "))
			require.Equal(t, expectedStr, actualStr, actualStr)
		})
	}
}

func TestImports(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []Import
	}{{
		name:   "parses a single import correctly",
		source: `import 'foo/bar/test.proto';`,
		want:   []Import{{Name: "foo/bar/test.proto", Public: false}},
	}, {
		name:   "parses public imports correctly",
		source: `import public "foo/bar/test.proto";`,
		want:   []Import{{Name: "foo/bar/test.proto", Public: true}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseString("test.proto", tt.source)
			require.NoError(t, err)
			clearPos(got)
			require.Equal(t, tt.want, got.Imports)
		})
	}
}

var zeroPos = reflect.ValueOf(lexer.Position{})

func clearPos(node any) {
	clearPosFromValue(reflect.ValueOf(node))
}

func clearPosFromValue(node reflect.Value) {
	node = reflect.Indirect(node)
	switch node.Kind() {
	case reflect.Array, reflect.Slice:
		for i := 0; i < node.Len(); i++ {
			clearPosFromValue(node.Index(i))
		}
	case reflect.Struct:
		for i := 0; i < node.NumField(); i++ {
			f := node.Field(i)
			if node.Type().Field(i).Name == "Pos" {
				f.Set(zeroPos)
				continue
			}
			if f.CanInterface() {
				clearPosFromValue(f)
			}
		}
	}
}

func toBig(n int) *big.Float {
	f, _, _ := big.ParseFloat(strconv.Itoa(n), 10, 64, 0)
	return f
}

func ptr[T any](v T) *T {
	vv := T(v)
	return &vv
}
