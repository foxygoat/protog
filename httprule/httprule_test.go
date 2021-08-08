package httprule

import (
	"strings"
	"testing"

	"foxygo.at/protog/httprule/internal"
	"github.com/stretchr/testify/require"
	pb "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
)

func TestValidateHTTPRule(t *testing.T) {
	r := &pb.HttpRule{
		Pattern: &pb.HttpRule_Get{Get: "abc"},
		Body:    "abc",
	}
	require.Error(t, ValidateHTTPRule(r))
	r.Body = "*"
	require.Error(t, ValidateHTTPRule(r))
	r.Body = ""
	require.NoError(t, ValidateHTTPRule(r))

	r = &pb.HttpRule{
		Pattern: &pb.HttpRule_Delete{Delete: "abc"},
		Body:    "abc",
	}
	require.Error(t, ValidateHTTPRule(r))
	r.Body = "*"
	require.Error(t, ValidateHTTPRule(r))
	r.Body = ""
	require.NoError(t, ValidateHTTPRule(r))
}

func requireProtoEqual(t *testing.T, want, got proto.Message) {
	t.Helper()
	require.True(t, proto.Equal(want, got), "protos are not Equal \nproto1: %v\nproto2: %v", want, got)
}

func requireProtoNotEqual(t *testing.T, want, got proto.Message) {
	t.Helper()
	require.False(t, proto.Equal(want, got), "protos are Equal \nproto1: %v\nproto2: %v", want, got)
}

func TestParseProtoResponseMsg1(t *testing.T) {
	rule := &pb.HttpRule{}
	s := `{"field1": "val1"}`
	got := &internal.TestMessage1{}
	want := &internal.TestMessage1{Field1: "val1"}
	err := ParseProtoResponse(rule, strings.NewReader(s), got)
	require.NoError(t, err)
	requireProtoEqual(t, want, got)

	s = `{"field1": "XXXX"}`
	err = ParseProtoResponse(rule, strings.NewReader(s), got)
	require.NoError(t, err)
	requireProtoNotEqual(t, want, got)
}

func TestParseProtoResponseMsg2(t *testing.T) {
	rule := &pb.HttpRule{}
	s := `{"field1": "val1"}`
	got := &internal.TestMessage2{}
	want := &internal.TestMessage2{Field1: "val1"}
	err := ParseProtoResponse(rule, strings.NewReader(s), got)
	require.NoError(t, err)
	requireProtoEqual(t, want, got)

	s = `{}`
	want = &internal.TestMessage2{}
	err = ParseProtoResponse(rule, strings.NewReader(s), got)
	require.NoError(t, err)
	requireProtoEqual(t, want, got)

	s = `{"field2": 0}`
	err = ParseProtoResponse(rule, strings.NewReader(s), got)
	require.NoError(t, err)
	requireProtoEqual(t, want, got)

	s = `{"field2": 3}`
	want = &internal.TestMessage2{Field2: 3}
	err = ParseProtoResponse(rule, strings.NewReader(s), got)
	require.NoError(t, err)
	requireProtoEqual(t, want, got)

	s = `{"field3Sub": {"subField": "abc"} }`
	want = &internal.TestMessage2{Field3Sub: &internal.SubMessage{SubField: "abc"}}
	err = ParseProtoResponse(rule, strings.NewReader(s), got)
	require.NoError(t, err)
	requireProtoEqual(t, want, got)
}

func TestField(t *testing.T) {
	m := &internal.TestMessage2{
		Field1:    "", // zero value
		Field2:    12,
		Field3Sub: nil,
	}
	got, err := newField("field3_sub", m)
	require.NoError(t, err)
	wantSub := &internal.SubMessage{}
	requireProtoEqual(t, wantSub, got)

	wantMsg := &internal.TestMessage2{
		Field2:    12,
		Field3Sub: &internal.SubMessage{},
	}
	requireProtoEqual(t, wantMsg, m)

	m = &internal.TestMessage2{
		Field3Sub: &internal.SubMessage{SubField: "abc"},
	}
	got, err = newField("field3_sub", m)
	wantMsg = &internal.TestMessage2{
		Field3Sub: &internal.SubMessage{},
	}
	require.NoError(t, err)
	requireProtoEqual(t, wantSub, got)
	requireProtoEqual(t, wantMsg, m)
}

func TestParseProtoResponseSub(t *testing.T) {
	rule := &pb.HttpRule{
		ResponseBody: "field3_sub",
	}
	s := `{"subField": "abc"}`
	got := &internal.TestMessage2{}
	want := &internal.TestMessage2{
		Field1:    "", // zero value
		Field2:    0,
		Field3Sub: &internal.SubMessage{SubField: "abc"},
	}
	err := ParseProtoResponse(rule, strings.NewReader(s), got)
	require.NoError(t, err)
	requireProtoEqual(t, want, got)
}

func TestParseProtoResponseErr(t *testing.T) {
	rule := &pb.HttpRule{
		Pattern: &pb.HttpRule_Get{Get: "/"},
		Body:    "*",
	}
	err := ParseProtoResponse(rule, strings.NewReader(""), nil)
	require.Error(t, err)

	rule = &pb.HttpRule{
		ResponseBody: "MISSING_FIELD",
	}
	m := &internal.TestMessage2{}
	err = ParseProtoResponse(rule, strings.NewReader("{}"), m)
	require.Error(t, err)

	rule = &pb.HttpRule{
		ResponseBody: "field2",
	}
	err = ParseProtoResponse(rule, strings.NewReader("{}"), m)
	require.Error(t, err)
}
