package httprule

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"foxygo.at/protog/httprule/internal"
	_ "foxygo.at/protog/httprule/internal"
	"github.com/stretchr/testify/require"
	pb "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetHttpRule(t *testing.T) {
	got, err := getHttpRule("/Echo/Hello")
	require.NoError(t, err)
	want := &pb.HttpRule{Pattern: &pb.HttpRule_Post{Post: "/api/echo/hello"}, Body: "*"}
	requireProtoEqual(t, want, got)
}

func TestEchoClient(t *testing.T) {
	ctx := context.Background()
	s := newTestServer(`{"response": "Hiya"}`, 200)
	defer s.Close()

	cc := NewClientConn(s.URL, WithHTTPClient(s.Client()))
	req := &internal.HelloRequest{Message: "hallo"}
	echoClient := internal.NewEchoClient(cc)

	got, err := echoClient.Hello(ctx, req)

	want := &internal.HelloResponse{Response: "Hiya"}
	require.NoError(t, err)
	requireProtoEqual(t, want, got)
}

func TestEchoClientErr(t *testing.T) {
	ctx := context.Background()
	s := newTestServer(`{"response": "Hiya"}`, 200)
	defer s.Close()

	cc := NewClientConn(s.URL, WithHTTPClient(s.Client()))
	req := &internal.HelloRequest{Message: "hallo"}
	resp := &internal.HelloResponse{}
	want := &internal.HelloResponse{Response: "Hiya"}

	// happy path
	err := cc.Invoke(ctx, "/Echo/Hello", req, resp)
	require.NoError(t, err)
	requireProtoEqual(t, want, resp)

	// errors
	err = cc.Invoke(ctx, "/Echo/MISSING_METHOD", req, resp)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrMethodNotFound)

	_, err = cc.NewStream(ctx, nil, "")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotImplemented)

	cc.BaseURL = "htt ps://badurl.com"
	err = cc.Invoke(ctx, "/Echo/Hello", req, resp)
	require.Error(t, err)
	targetErr := &url.Error{}
	require.ErrorAs(t, err, &targetErr)
}

func TestEchoClientStatusErr(t *testing.T) {
	ctx := context.Background()
	s := newTestServer("", http.StatusBadRequest)
	defer s.Close()
	cc := NewClientConn(s.URL, WithHTTPClient(s.Client()))
	echoClient := internal.NewEchoClient(cc)
	_, err := echoClient.Hello(ctx, &internal.HelloRequest{Message: ""})
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestWithHeader(t *testing.T) {
	ctx := context.Background()
	s := newTestServer(`{"response": "Hiya"}`, 200)
	defer s.Close()

	cc := NewClientConn(s.URL, WithHTTPClient(s.Client()), WithHeader("Hello", "World"))
	req := &internal.HelloRequest{Message: "hallo"}
	echoClient := internal.NewEchoClient(cc)

	got, err := echoClient.Hello(ctx, req)

	want := &internal.HelloResponse{Response: "Hiya"}
	require.NoError(t, err)
	requireProtoEqual(t, want, got)
	require.Equal(t, "World", s.request.Header.Get("Hello"))
}

func TestGetHttpRuleErr(t *testing.T) {
	_, err := getHttpRule("/tomany/slashes///")
	require.ErrorIs(t, err, ErrInvalidMethod)

	_, err = getHttpRule("/MISSING/Hello")
	require.ErrorIs(t, err, ErrServiceNotFound)

	_, err = getHttpRule("/HelloRequest/message")
	require.ErrorIs(t, err, ErrInvalidMethod)

	_, err = getHttpRule("/Echo/Hello2")
	require.ErrorIs(t, err, ErrHttpRuleNotFound)
}

func TestErrorStatus(t *testing.T) {
	require.Nil(t, errorStatus(http.StatusOK))

	err := errorStatus(http.StatusUnauthorized)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	err = errorStatus(http.StatusUnauthorized)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	err = errorStatus(http.StatusForbidden)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	err = errorStatus(http.StatusNotFound)
	require.Equal(t, codes.Unimplemented, status.Code(err))

	err = errorStatus(http.StatusTooManyRequests)
	require.Equal(t, codes.Unavailable, status.Code(err))

	err = errorStatus(-1)
	require.Equal(t, codes.Unknown, status.Code(err))
}

type testServer struct {
	*httptest.Server
	// request is cloned from the request received by the test handler.
	// It can be used to test that a client set correct headers, etc.
	request *http.Request
}

func newTestServer(body string, statusCode int) *testServer {
	mux := http.NewServeMux()
	ts := &testServer{Server: httptest.NewServer(mux)}
	h := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(statusCode)
		fmt.Fprintln(w, body)
		ts.request = req.Clone(context.Background())
	})
	p := "/api/echo/hello"
	mux.HandleFunc(p, h)
	return ts
}
