// Package httprule provides utilities to map google.api.http annotation
// to net/http Request and Response types. These utilities allow to
// generate HTTP Clients for a given proto service. The methods of this
// service have their HTTP mappings specified via `google.api.http`
// method options, e.g.:
//
// service HelloService {
//     rpc Hello (HelloRequest) returns (HelloResponse) {
//         option (google.api.http) = { post:"/api/hello" body:"*" };
//     };
// };
//
// HttpRule proto:   https://github.com/googleapis/googleapis/blob/master/google/api/http.proto
// HttpRule codegen: https://pkg.go.dev/google.golang.org/genproto/googleapis/api/annotations
package httprule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	pb "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	ErrInvalidHttpRule = fmt.Errorf("invalid HttpRule")
)

// ParseProtoResponse parses a proto message from a HTTPRule and an
// io.Reader, typically an http.Response.Body. rule.ResponseBody may
// only reference a top-level field of the response target proto
// message. rule.ResponseBody may also be empty.
func ParseProtoResponse(rule *pb.HttpRule, body io.Reader, target proto.Message) error {
	if err := ValidateHTTPRule(rule); err != nil {
		return err
	}

	b, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("ParseProtoResponse: %w", err)
	}
	if rule.ResponseBody != "" {
		target, err = newField(rule.ResponseBody, target)
		if err != nil {
			return err
		}
	}

	if err := protojson.Unmarshal(b, target); err != nil {
		return fmt.Errorf("cannot protojson unmarshal: %w", err)
	}
	return nil
}

func newField(fieldName string, msg proto.Message) (proto.Message, error) {
	m := msg.ProtoReflect()
	fd := m.Descriptor().Fields().ByTextName(fieldName)
	if fd == nil {
		return nil, fmt.Errorf("%w: field '%s' not in message", ErrInvalidHttpRule, fieldName)
	}
	if fd.Kind() != protoreflect.MessageKind {
		return nil, fmt.Errorf("%w: field '%s' is not a message type", ErrInvalidHttpRule, fieldName)
	}
	val := m.NewField(fd)
	m.Set(fd, val)
	return val.Message().Interface(), nil
}

func ValidateHTTPRule(rule *pb.HttpRule) error {
	if method(rule) == "" {
		return fmt.Errorf("%w: invalid method or empty path", ErrInvalidHttpRule)
	}
	if (rule.GetGet() != "" || rule.GetDelete() != "") && rule.Body != "" {
		return fmt.Errorf("%w: body not allowed with GET or DELETE request", ErrInvalidHttpRule)
	}
	return nil
}

// NewHTTPReuqest creates an *http.Request for a given proto message and
// HTTPRule, containing the request mapping information.
func NewHTTPRequest(rule *pb.HttpRule, baseURL string, req proto.Message) (*http.Request, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse baseURL: %w", err)
	}
	if err := ValidateHTTPRule(rule); err != nil {
		return nil, err
	}

	templPath := templatePath(rule) // e.g. /v1/messages/{message_id}/{sub.subfield}
	keys := map[string]bool{}
	p, err := interpolate(templPath, req, keys)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, p)

	body, err := jsonBody(rule.Body, req, keys)
	if err != nil {
		return nil, err
	}
	header, err := requestHeaders(rule, req, keys)
	if err != nil {
		return nil, err
	}
	u.RawQuery, err = urlRawQuery(rule.Body, req, keys)
	if err != nil {
		return nil, err
	}

	r, err := http.NewRequest(method(rule), u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %w", err)
	}
	r.Header = header
	return r, nil
}

func requestHeaders(httpRule *pb.HttpRule, req proto.Message, skip map[string]bool) (http.Header, error) {
	h := http.Header{}
	for _, rule := range httpRule.AdditionalBindings {
		if custom := rule.GetCustom(); custom != nil {
			if custom.Kind == "header" {
				key, val, err := parseHeader(custom.Path, req, skip)
				if err != nil {
					return nil, err
				}
				h.Add(key, val)
			}
		}
	}
	return h, nil
}

func parseHeader(s string, m proto.Message, skip map[string]bool) (key string, val string, err error) {
	// "Content-Type: application/json"
	parts := strings.SplitN(s, ":", 2)
	key, val = parts[0], strings.TrimSpace(parts[1])
	key = http.CanonicalHeaderKey(key)
	val, err = interpolate(val, m, skip)
	return key, val, err
}

// jsonBody returns an io.Reader of the for the given top-level message field, or the whole message
// if bodyField is set to "*".
func jsonBody(bodyField string, msg proto.Message, skip map[string]bool) (io.Reader, error) {
	if bodyField == "" {
		return nil, nil
	}
	if (bodyField == "*" && len(skip) != 0) || skip[bodyField] {
		return nil, fmt.Errorf("%w: body and path fields overlap", ErrInvalidHttpRule)
	}
	if bodyField != "*" {
		m := msg.ProtoReflect()
		fds := m.Descriptor().Fields()
		fd := fds.ByTextName(bodyField)
		if fd == nil {
			return nil, fmt.Errorf("%w: field '%s' not in message", ErrInvalidHttpRule, bodyField)
		}
		if fd.Kind() != protoreflect.MessageKind {
			return nil, fmt.Errorf("%w: field '%s' is not a message type", ErrInvalidHttpRule, bodyField)
		}
		skip[bodyField] = true
		msg = m.Get(fd).Message().Interface()
	}
	b, err := protojson.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("cannot create bodyJSON for '%s': %w", bodyField, err)
	}
	return bytes.NewReader(b), nil
}

// interpolate returns a path from a templated path and a proto message
// whose values are substituted in the path template. For example:
//
//    templatePath:              "/v1/messages/{message_id}"
//    proto message definition:  message M {  string message_id = 1; }
//    proto message value:       { message_id: 123 }
//
//    => result path:            "/v1/messages/123"
//
// Referenced message fields must have primitive types; they cannot not
// repeated or message types. See:
// https://github.com/googleapis/googleapis/blob/master/google/api/http.proto
//
// Only basic substitutions via {var}, {var=*} and {var=**} of top-level
// fields are supported. {var} is a short hand for {var=*} and
// substitutes the value of a message field with path escaping (%2...).
// {var=**} will substitute without path. This may be useful for
// expansions where the values include slashes and is deviation from
// the spec, which only allows {var=**} for the last path segment.
//
// The extended syntax for `*` and `**` substitutions with further path
// segments is not implemented. Nested field values are not supported
// (e.g.{msg_field.sub_field}).
//
// TODO: Complete interpolate implementation for full substitution grammar
func interpolate(templ string, msg proto.Message, skipKeys map[string]bool) (string, error) {
	m := msg.ProtoReflect()
	fds := m.Descriptor().Fields()
	re := regexp.MustCompile(`{([a-zA-Z0-9_-]+)(=\*\*?)?}`)

	result := templ
	for _, match := range re.FindAllStringSubmatch(templ, -1) {
		fullMatch, fieldName := match[0], match[1]
		if skipKeys[fieldName] {
			return "", fmt.Errorf("%w: field %q already in use", ErrInvalidHttpRule, fieldName)
		}
		fd := fds.ByTextName(fieldName)
		if fd == nil {
			return "", fmt.Errorf("cannot find %s in request proto message: %w", fieldName, ErrInvalidHttpRule)
		}
		if fd.Kind() == protoreflect.MessageKind || fd.Cardinality() == protoreflect.Repeated {
			return "", fmt.Errorf("only primitive types supported in path substitution")
		}
		val := m.Get(fd).String()
		if match[2] != "=**" {
			val = url.PathEscape(val)
		}
		result = strings.ReplaceAll(result, fullMatch, val)
		skipKeys[fieldName] = true
	}
	return result, nil
}

// urlRawQuery converts a proto message into url.Values.
//
//  {"a": "A", "b": {"nested": "🐣"}, "SLICE": [1, 2]}}
//       => ?a=A&b.nested=🐣&SLICE=1&SLICE=2
//
// TODO: Investigate zero value encoding for optional and default types.
func urlRawQuery(bodyRule string, m proto.Message, skip map[string]bool) (string, error) {
	if bodyRule == "*" {
		return "", nil
	}
	pm := &protojson.MarshalOptions{UseProtoNames: true}
	b, err := pm.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("cannot marshal message for URL query encoding: %w", err)
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(b, &obj); err != nil {
		return "", fmt.Errorf("cannot query encode: error unmarshalling '%v': %w", m, err)
	}

	vals := url.Values{}
	if err := queryEnc(obj, "", vals, skip); err != nil {
		return "", err
	}
	return vals.Encode(), nil
}

func queryEnc(m map[string]interface{}, path string, vals url.Values, skip map[string]bool) error {
	for key, val := range m {
		p := path + key
		if skip[p] {
			continue
		}
		switch v := val.(type) {
		case int, bool, string, float64:
			vals.Add(p, fmt.Sprintf("%v", v))
		case []interface{}:
			if err := addSlice(v, p, vals); err != nil {
				return err
			}
		case map[string]interface{}:
			if err := queryEnc(v, p+".", vals, skip); err != nil {
				return err
			}
		default:
			return fmt.Errorf("cannot query encode %T", v)
		}
	}
	return nil
}

func addSlice(s []interface{}, path string, vals url.Values) error {
	for _, el := range s {
		switch v := el.(type) {
		case int, bool, string, float64:
			vals.Add(path, fmt.Sprintf("%v", v))
		default:
			return fmt.Errorf("cannot query encode slices of non-basic type %T", v)
		}
	}
	return nil
}

func templatePath(rule *pb.HttpRule) string {
	switch {
	case rule.GetGet() != "":
		return rule.GetGet()
	case rule.GetPut() != "":
		return rule.GetPut()
	case rule.GetPost() != "":
		return rule.GetPost()
	case rule.GetDelete() != "":
		return rule.GetDelete()
	}
	return ""
}

func method(rule *pb.HttpRule) string {
	switch {
	case rule.GetGet() != "":
		return http.MethodGet
	case rule.GetPut() != "":
		return http.MethodPut
	case rule.GetPost() != "":
		return http.MethodPost
	case rule.GetDelete() != "":
		return http.MethodDelete
	case rule.GetPatch() != "":
		return http.MethodPatch
	}
	return ""
}
