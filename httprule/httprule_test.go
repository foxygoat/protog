package httprule

import (
	"io"
	"net/http"
	"net/url"
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
	require.NoError(t, ValidateHTTPRule(r))
	r.Body = "*"
	require.NoError(t, ValidateHTTPRule(r))
	r.Body = ""
	require.NoError(t, ValidateHTTPRule(r))

	r = &pb.HttpRule{
		Pattern: &pb.HttpRule_Delete{Delete: "abc"},
		Body:    "abc",
	}
	require.NoError(t, ValidateHTTPRule(r))
	r.Body = "*"
	require.NoError(t, ValidateHTTPRule(r))
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
	rule := &pb.HttpRule{Pattern: &pb.HttpRule_Get{Get: "/"}}
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
	rule := &pb.HttpRule{Pattern: &pb.HttpRule_Get{Get: "/"}}
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
		Pattern:      &pb.HttpRule_Get{Get: "/"},
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
	m := &internal.TestMessage2{}
	err := ParseProtoResponse(rule, strings.NewReader("{ BAD JSON"), m)
	require.Error(t, err)

	rule = &pb.HttpRule{
		Pattern:      &pb.HttpRule_Get{Get: "/"},
		ResponseBody: "MISSING_FIELD",
	}
	m = &internal.TestMessage2{}
	err = ParseProtoResponse(rule, strings.NewReader("{}"), m)
	require.Error(t, err)

	rule = &pb.HttpRule{
		Pattern:      &pb.HttpRule_Get{Get: "/"},
		ResponseBody: "field2",
	}
	err = ParseProtoResponse(rule, strings.NewReader("{}"), m)
	require.Error(t, err)
}

func TestNewHTTPRequest(t *testing.T) {
	u1 := "https://exaple.com"
	u2 := "https://exaple.com/"
	u3 := "https://exaple.com/api"
	u4 := "https://exaple.com/api/"
	tests := map[string]struct {
		rule       *pb.HttpRule
		baseURL    string
		pbReq      proto.Message
		wantMethod string
		wantURL    string
		wantBody   string
		wantHeader http.Header
	}{
		"simple-query": {
			rule:       &pb.HttpRule{Pattern: &pb.HttpRule_Get{Get: "/"}},
			baseURL:    u2,
			pbReq:      &internal.TestMessage1{Field1: "val1"},
			wantMethod: "GET",
			wantURL:    u2 + "?field1=val1"},
		"simple-path": {
			rule:       &pb.HttpRule{Pattern: &pb.HttpRule_Delete{Delete: "v1/{field1}"}},
			baseURL:    u1,
			pbReq:      &internal.TestMessage1{Field1: "val1"},
			wantMethod: "DELETE",
			wantURL:    u1 + "/v1/val1"},
		"path_and_query": {
			rule:       &pb.HttpRule{Pattern: &pb.HttpRule_Post{Post: "v1/{weird_FieldName_1_=*}/bool/{a_bool2}"}},
			baseURL:    u3,
			pbReq:      &internal.TestMessage3{Weird_FieldName_1_: "val1", ABool2: true, AInt_3: 2},
			wantMethod: "POST",
			wantURL:    u3 + "/v1/val1/bool/true?a_int_3=2"},
		"path_zero_values": {
			rule:       &pb.HttpRule{Pattern: &pb.HttpRule_Put{Put: "v1/{weird_FieldName_1_}/bool/{a_bool2=**}"}},
			baseURL:    u4,
			pbReq:      &internal.TestMessage3{},
			wantMethod: "PUT",
			wantURL:    u4 + "v1/bool/false"},
		"path_with_slash": {
			rule:       &pb.HttpRule{Pattern: &pb.HttpRule_Put{Put: "{field1=*}"}},
			baseURL:    u4,
			pbReq:      &internal.TestMessage1{Field1: "library/ubuntu"},
			wantMethod: "PUT",
			wantURL:    u4 + "library%252Fubuntu"},
		"path_with_slash_unescaped": {
			rule:       &pb.HttpRule{Pattern: &pb.HttpRule_Put{Put: "{field1=**}"}},
			baseURL:    u4,
			pbReq:      &internal.TestMessage1{Field1: "library/ubuntu"},
			wantMethod: "PUT",
			wantURL:    u4 + "library/ubuntu"},
		"path_encoding": {
			rule:       &pb.HttpRule{Pattern: &pb.HttpRule_Put{Put: "{field1}"}},
			baseURL:    u4,
			pbReq:      &internal.TestMessage1{Field1: "path with whitespace"},
			wantMethod: "PUT",
			wantURL:    u4 + "path%2520with%2520whitespace"},
		"query-encoding": {
			rule:       &pb.HttpRule{Pattern: &pb.HttpRule_Get{Get: "/"}},
			baseURL:    u2,
			pbReq:      &internal.TestMessage1{Field1: "query with whitespace"},
			wantMethod: "GET",
			wantURL:    u2 + "?field1=query+with+whitespace"},
		"query-with-subfields": {
			rule:    &pb.HttpRule{Pattern: &pb.HttpRule_Patch{Patch: "/"}},
			baseURL: u3,
			pbReq: &internal.TestMessage2{
				Field1: "A",
				Field3Sub: &internal.SubMessage{
					SubField:  "B",
					SubRepeat: []int32{1, 2},
				},
			},
			wantMethod: "PATCH",
			wantURL:    u3 + "?field1=A&field3_sub.sub_field=B&field3_sub.sub_repeat=1&field3_sub.sub_repeat=2"},
		"simple-body": {
			rule:       &pb.HttpRule{Pattern: &pb.HttpRule_Post{Post: "/"}, Body: "*"},
			baseURL:    u3,
			pbReq:      &internal.TestMessage1{Field1: "val1"},
			wantMethod: "POST",
			wantURL:    u3,
			wantBody:   `{"field1": "val1"}`},
		"body2": {
			rule:    &pb.HttpRule{Pattern: &pb.HttpRule_Post{Post: "/"}, Body: "*"},
			baseURL: u3,
			pbReq: &internal.TestMessage2{
				Field1: "A",
				Field3Sub: &internal.SubMessage{
					SubField:  "B",
					SubRepeat: []int32{1, 2},
				},
			},
			wantMethod: "POST",
			wantURL:    u3,
			wantBody:   `{"field1": "A", "field3Sub": { "subField": "B", "subRepeat": [1, 2]} }`},
		"body3": {
			rule:    &pb.HttpRule{Pattern: &pb.HttpRule_Post{Post: "/"}, Body: "*"},
			baseURL: u3,
			pbReq: &internal.TestMessage3{
				Weird_FieldName_1_: "A",
				ABool2:             false,
				AInt_3:             4,
				ARepeat:            []string{},
			},
			wantMethod: "POST",
			wantURL:    u3,
			wantBody:   `{"weirdFieldName1": "A", "aInt3": 4} `},
		"partial-body": {
			rule:    &pb.HttpRule{Pattern: &pb.HttpRule_Post{Post: "/field/{field1}/"}, Body: "field3_sub"},
			baseURL: u3,
			pbReq: &internal.TestMessage2{
				Field1: "A",
				Field2: 22,
				Field3Sub: &internal.SubMessage{
					SubField:  "B",
					SubRepeat: []int32{1, 10},
				},
			},
			wantMethod: "POST",
			wantURL:    u3 + "/field/A?field2=22",
			wantBody:   `{"subField": "B", "subRepeat": [1, 10]} `},
		"header": {
			rule: &pb.HttpRule{
				Pattern: &pb.HttpRule_Post{Post: "/"},
				Body:    "field3_sub",
				AdditionalBindings: []*pb.HttpRule{
					{
						Pattern: &pb.HttpRule_Custom{
							Custom: &pb.CustomHttpPattern{
								Kind: "header",
								Path: "field2: {field2}",
							},
						},
					},
				},
			},
			baseURL: u3,
			pbReq: &internal.TestMessage2{
				Field1: "val1",
				Field2: 2,
				Field3Sub: &internal.SubMessage{
					SubField:  "sub1",
					SubRepeat: []int32{1, 2, 3},
				},
			},
			wantHeader: http.Header{"Field2": []string{"2"}},
			wantMethod: "POST",
			wantURL:    u3 + "?field1=val1",
			wantBody: `{
				"subField": "sub1",
				"subRepeat": [1,2,3]
			}`},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			got, err := NewHTTPRequest(tc.rule, tc.baseURL, tc.pbReq)
			require.NoError(t, err)
			require.Equal(t, tc.wantMethod, got.Method)
			require.Equal(t, tc.wantURL, got.URL.String())
			if tc.wantBody == "" {
				require.Nil(t, got.Body)
			} else {
				b, err := io.ReadAll(got.Body)
				require.NoError(t, err)
				got.Body.Close()
				require.JSONEq(t, tc.wantBody, string(b))
			}
			if tc.wantHeader != nil {
				require.Equal(t, tc.wantHeader, got.Header)
			}
		})
	}
}

func TestNewHTTPRequestErr(t *testing.T) {
	u := "https://exaple.com/api/"
	tests := map[string]struct {
		rule    *pb.HttpRule
		baseURL string
		pbReq   proto.Message
	}{
		"invalid-url": {
			rule:    &pb.HttpRule{Pattern: &pb.HttpRule_Get{Get: "/"}},
			baseURL: "htt  s://BAD-URL",
			pbReq:   &internal.TestMessage1{}},
		"invalid-http-rule": {
			rule:    &pb.HttpRule{},
			baseURL: u,
			pbReq:   &internal.TestMessage1{}},
		"invalid-http-rule-pattern": {
			rule:    &pb.HttpRule{},
			baseURL: u,
			pbReq:   &internal.TestMessage1{}},
		"no-match-path": {
			rule:    &pb.HttpRule{Pattern: &pb.HttpRule_Get{Get: "/{MISSING}/"}},
			baseURL: u,
			pbReq:   &internal.TestMessage2{}},
		"no-primitive-match-path": {
			rule:    &pb.HttpRule{Pattern: &pb.HttpRule_Get{Get: "/{field3_sub}/"}},
			baseURL: u,
			pbReq:   &internal.TestMessage2{}},
		"no-primitive-match-path2": {
			rule:    &pb.HttpRule{Pattern: &pb.HttpRule_Get{Get: "/{a_repeat}/"}},
			baseURL: u,
			pbReq:   &internal.TestMessage3{ARepeat: []string{"A", "B"}}},
		"invalid-query": {
			rule:    &pb.HttpRule{Pattern: &pb.HttpRule_Get{Get: "/"}},
			baseURL: u,
			pbReq: &internal.TestMessage3{
				ASubmsgRepeat: []*internal.SubMessage{{SubField: "sub"}},
			}},
		"invalid-body-field": {
			rule:    &pb.HttpRule{Pattern: &pb.HttpRule_Post{Post: "/"}, Body: "MISSING"},
			baseURL: u,
			pbReq:   &internal.TestMessage1{}},
		"invalid-body-field-type": {
			rule:    &pb.HttpRule{Pattern: &pb.HttpRule_Post{Post: "/"}, Body: "field1"},
			baseURL: u,
			pbReq:   &internal.TestMessage1{}},
		"path-body-field-overlap": {
			rule:    &pb.HttpRule{Pattern: &pb.HttpRule_Post{Post: "v1/{field1}"}, Body: "*"},
			baseURL: u,
			pbReq:   &internal.TestMessage1{Field1: "val1"}},
		"path-header-overlap": {
			rule: &pb.HttpRule{
				Pattern: &pb.HttpRule_Post{Post: "v1/{field1}"},
				AdditionalBindings: []*pb.HttpRule{
					{
						Pattern: &pb.HttpRule_Custom{
							Custom: &pb.CustomHttpPattern{
								Kind: "header",
								Path: "field1: {field1}",
							},
						},
					},
				},
			},
			baseURL: u,
			pbReq:   &internal.TestMessage1{Field1: "val1"}},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			_, err := NewHTTPRequest(tc.rule, tc.baseURL, tc.pbReq)
			require.Error(t, err)
		})
	}
}

func TestQueryEncErr(t *testing.T) {
	skip := map[string]bool{}
	urlVals := url.Values{}
	path := ""
	invalidMsg := map[string]interface{}{
		"nested": map[string]interface{}{
			"f": func() {},
		},
	}
	err := queryEnc(invalidMsg, path, urlVals, skip)
	require.Error(t, err)
}
