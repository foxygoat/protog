package httprule

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	pb "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	descpb "google.golang.org/protobuf/types/descriptorpb"
)

type ClientConn struct {
	HTTPClient *http.Client
	BaseURL    string

	// header contains HTTP headers added to every HTTP request made via
	// this ClientConn.
	header http.Header
}

// Option is a function option for customising a httprule.ClientConn via
// the httprule.NewClientConn() constructor. They are typically created
// by the With* functions in this package.
type Option func(*ClientConn)

var (
	ErrInvalidMethod    = errors.New("invalid gRPC method string")
	ErrMethodNotFound   = errors.New("method not found")
	ErrServiceNotFound  = errors.New("service not found")
	ErrNotImplemented   = errors.New("not implemented")
	ErrHttpRuleNotFound = errors.New("no HttpRule")
)

// NewClientConn creates an httprule.ClientConn for making HTTP requests mapped
// from gRPC method calls annotated with google.api.http annotations. The
// baseURL is used as a prefix to the paths specified in annotations and is
// typically just a scheme and host, however all elements of the baseURL are
// preserved and the path in the annotation is appended to the path in the
// baseURL.
//
// A zero-value http.Client will be used by the ClientConn for all HTTP
// requests it makes. An alternate http.Client can be provided as an
// Option.
//
// opts are function options that modify the http.ClientConn and are typically
// created by the With* functions in this package.
func NewClientConn(baseURL string, opts ...Option) *ClientConn {
	cc := &ClientConn{
		HTTPClient: &http.Client{},
		BaseURL:    baseURL,
		header:     http.Header{},
	}
	for _, opt := range opts {
		opt(cc)
	}
	return cc
}

// WithHTTPClient retuens an httprule.Option for setting the HTTP client
// used by a ClientConn.
func WithHTTPClient(client *http.Client) Option {
	return func(cc *ClientConn) {
		cc.HTTPClient = client
	}
}

// WithHeader returns an httprule.Option for setting a header on each HTTP
// request made by a httprule.ClientConn. The header is added to any that may
// be specified by a google.api.http annotation on a method. The returned
// option can be passed to the NewClientConn() constructor.
func WithHeader(key, value string) Option {
	return func(cc *ClientConn) {
		cc.header.Set(key, value)
	}
}

func (c *ClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, ErrNotImplemented
}

func (c *ClientConn) Invoke(ctx context.Context, method string, args, reply interface{}, opts ...grpc.CallOption) error {
	rule, err := getHttpRule(method)
	if err != nil {
		return err
	}
	req, err := NewHTTPRequest(rule, c.BaseURL, args.(proto.Message))
	if err != nil {
		return err
	}
	for key, vals := range c.header {
		for _, v := range vals {
			req.Header.Add(key, v)
		}
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := errorStatus(resp.StatusCode); err != nil {
		return err
	}
	return ParseProtoResponse(rule, resp, reply.(proto.Message))
}

func getHttpRule(method string) (*pb.HttpRule, error) {
	parts := strings.Split(method, "/")
	if len(parts) != 3 || parts[0] != "" {
		return nil, fmt.Errorf("%w: %s", ErrInvalidMethod, method)
	}

	serviceName, methodName := protoreflect.FullName(parts[1]), protoreflect.Name(parts[2])
	desc, err := protoregistry.GlobalFiles.FindDescriptorByName(serviceName)
	if err != nil {
		return nil, fmt.Errorf("%w, %v", ErrServiceNotFound, err)
	}

	sd, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%w: '%s' is not a service", ErrInvalidMethod, serviceName)
	}

	md := sd.Methods().ByName(methodName)
	if md == nil {
		return nil, fmt.Errorf("%w: %s", ErrMethodNotFound, method)
	}

	mo, ok := md.Options().(*descpb.MethodOptions)
	if !ok {
		return nil, fmt.Errorf("method options are not MethodOptions")
	}

	if !proto.HasExtension(mo, pb.E_Http) {
		return nil, ErrHttpRuleNotFound
	}
	v := proto.GetExtension(mo, pb.E_Http)
	httpRule, ok := v.(*pb.HttpRule)
	if !ok {
		return nil, fmt.Errorf("HttpRule is not HttpRule")
	}
	return httpRule, nil
}

// errorStatus maps HTTP status code to gRPC status as per
// https://grpc.github.io/grpc/core/md_doc_http-grpc-status-mapping.html
// An alternate extended mapping could be derived from
// https://github.com/grpc-ecosystem/grpc-gateway/blob/master/runtime/errors.go#L36
func errorStatus(statusCode int) error {
	if 200 <= statusCode && statusCode <= 399 {
		return nil
	}
	switch statusCode {
	case http.StatusBadRequest:
		return status.Error(codes.Internal, "")
	case http.StatusUnauthorized:
		return status.Error(codes.Unauthenticated, "")
	case http.StatusForbidden:
		return status.Error(codes.PermissionDenied, "")
	case http.StatusNotFound:
		return status.Error(codes.Unimplemented, "")
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return status.Error(codes.Unavailable, "")
	}
	return status.Error(codes.Unknown, "")
}
