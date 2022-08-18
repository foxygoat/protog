package registry

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	_ "google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestCloneTypes(t *testing.T) {
	want := protoregistry.GlobalTypes
	got := CloneTypes(want)
	require.Exactly(t, want, got)
}

func TestAddDynamicTypes(t *testing.T) {
	fds := newFDS(t)

	types := &protoregistry.Types{}
	err := AddDynamicTypes(types, fds)
	require.NoError(t, err)

	// regtest.pb includes descriptor.proto, annotations.proto, http.proto and empty.proto
	// regtest: 3 messages, 0 enums, 4 extensions
	// descriptor.proto: 27 messages, 7 enums, 0 extensions
	// annotations.proto: 0 messages, 0 enums, 1 extension
	// http.proto: 3 messages, 0 enums, 0 extensions
	// empty.proto: 1 message, 0 enums, 0 extensions
	// total: 34 messages, 7 enums, 5 extensions
	require.Equal(t, 34, types.NumMessages())
	require.Equal(t, 7, types.NumEnums())
	require.Equal(t, 5, types.NumExtensions())

	// descriptorpb.FileDescriptorSet should be dynamic as we have only loaded
	// dynamic messages into the Types registry
	mt, err := types.FindMessageByName("google.protobuf.FileDescriptorSet")
	require.NoError(t, err)
	_, ok := mt.New().Interface().(*dynamicpb.Message)
	require.True(t, ok, "FileDescriptorSet is not dynamicpb.Message type")

}

func TestAddDynamicTypesKeepsConcrete(t *testing.T) {
	fds := newFDS(t)

	types := CloneTypes(protoregistry.GlobalTypes)
	err := AddDynamicTypes(types, fds)
	require.NoError(t, err)

	// descriptorpb.FileDescriptorSet was imported so should be a concrete type
	mt, err := types.FindMessageByName("google.protobuf.FileDescriptorSet")
	require.NoError(t, err)
	_, ok := mt.New().Interface().(*descriptorpb.FileDescriptorSet)
	require.True(t, ok, "FileDescriptorSet is not concrete type")

	// regtest.BaseMessage should be dynamic
	mt, err = types.FindMessageByName("regtest.BaseMessage")
	require.NoError(t, err)
	_, ok = mt.New().Interface().(*dynamicpb.Message)
	require.True(t, ok, "BaseMessage is not dynamicpb.Message type")
}

func newFDS(t *testing.T) *descriptorpb.FileDescriptorSet {
	t.Helper()
	b, err := os.ReadFile("testdata/regtest.pb")
	require.NoError(t, err)
	fds := descriptorpb.FileDescriptorSet{}
	err = proto.Unmarshal(b, &fds)
	require.NoError(t, err)
	return &fds
}
