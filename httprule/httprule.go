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
	"fmt"
	"io"

	pb "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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
	if fd.Message() == nil {
		return nil, fmt.Errorf("%w: field '%s' is not a message type", ErrInvalidHttpRule, fieldName)
	}
	val := m.NewField(fd)
	m.Set(fd, val)
	return val.Message().Interface(), nil
}

func ValidateHTTPRule(rule *pb.HttpRule) error {
	if (rule.GetGet() != "" || rule.GetDelete() != "") && rule.Body != "" {
		return fmt.Errorf("%w: body not allowed with GET or DELETE request", ErrInvalidHttpRule)
	}
	return nil
}
