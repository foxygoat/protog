package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func newFDS(t *testing.T, filename string) *descriptorpb.FileDescriptorSet {
	t.Helper()
	b, err := os.ReadFile(filename)
	require.NoError(t, err)
	fds := descriptorpb.FileDescriptorSet{}
	err = proto.Unmarshal(b, &fds)
	require.NoError(t, err)
	return &fds
}

func TestRunJSON(t *testing.T) {
	tmpDir := t.TempDir()
	fds := newFDS(t, "testdata/pbtest.pb")

	cli := PBConfig{
		Protoset:    fds,
		Out:         filepath.Join(tmpDir, "out.json"),
		MessageType: "BaseMessage",
		In:          `{"f": "F" }`,
	}

	formats := []string{"json", "j", ""}
	for _, format := range formats {
		t.Run("format-"+format, func(t *testing.T) {
			cli.OutFormat = format
			require.NoError(t, cli.Run())
			want := `{"f": "F" }`
			requireJSONFileContent(t, want, cli.Out)
		})
	}
}

func TestRunJSONZero(t *testing.T) {
	tmpDir := t.TempDir()
	fds := newFDS(t, "testdata/pbtest.pb")

	cli := PBConfig{
		Protoset:    fds,
		Out:         filepath.Join(tmpDir, "out.json"),
		MessageType: "BaseMessage",
		In:          `{"f": "" }`,
		Zero:        true,
		OutFormat:   "json",
	}
	require.NoError(t, cli.Run())
	want := `{"f": "" }`
	requireJSONFileContent(t, want, cli.Out)

	cli.Zero = false
	require.NoError(t, cli.Run())
	want = `{}`
	requireJSONFileContent(t, want, cli.Out)
}

func TestRunPrototext(t *testing.T) {
	tmpDir := t.TempDir()
	fds := newFDS(t, "testdata/pbtest.pb")

	cli := PBConfig{
		Protoset:    fds,
		Out:         filepath.Join(tmpDir, "out.txt"),
		MessageType: "BaseMessage",
		In:          `{"f": "F" }`,
	}
	formats := []string{"txt", "t", "prototxt"}
	for _, format := range formats {
		t.Run("format-"+format, func(t *testing.T) {
			cli.OutFormat = format
			require.NoError(t, cli.Run())
			want := `f:"F"` + "\n"
			out := filepath.Join(tmpDir, "out.txt")
			b, err := os.ReadFile(out)
			require.NoError(t, err)
			// prototext is made unstable with random whitespace. stabilize for this basic test.
			got := strings.ReplaceAll(string(b), " ", "")
			require.Equal(t, want, got)
		})
	}
}

func TestRunMessages(t *testing.T) {
	tmpDir := t.TempDir()
	fds := newFDS(t, "testdata/pbtest.pb")

	cli := PBConfig{
		Protoset: fds,
		Out:      filepath.Join(tmpDir, "out.json"),
		In:       `{"f": "F" }`,
	}
	messageTypes := []string{"BaseMessage", "pbtest.BaseMessage", ".pbtest.BaseMessage", "basemessage"}
	for _, messageType := range messageTypes {
		t.Run("message-"+messageType, func(t *testing.T) {
			cli.MessageType = messageType
			require.NoError(t, cli.Run())
			want := `{"f": "F" }`
			requireJSONFileContent(t, want, cli.Out)
		})
	}
}

func TestRunMessageErr(t *testing.T) {
	tmpDir := t.TempDir()
	fds := newFDS(t, "testdata/pbtest.pb")

	cli := PBConfig{
		Protoset: fds,
		Out:      filepath.Join(tmpDir, "out.json"),
		In:       `{"f": "F" }`,
	}
	messageTypes := []string{"Message", "..pbtest.BaseMessage"}
	for _, messageType := range messageTypes {
		t.Run("message-"+messageType, func(t *testing.T) {
			cli.MessageType = messageType
			require.Error(t, cli.Run())
		})
	}
}

func TestRunInErr(t *testing.T) {
	tmpDir := t.TempDir()
	fds := newFDS(t, "testdata/pbtest.pb")

	cli := PBConfig{
		Protoset:    fds,
		Out:         filepath.Join(tmpDir, "out.json"),
		MessageType: "BaseMessage",
		In:          `{"MISSING": "F" }`,
	}
	require.Error(t, cli.Run())
}

func TestWellKnown(t *testing.T) {
	tmpDir := t.TempDir()
	cli := PBConfig{
		Out:         filepath.Join(tmpDir, "out.json"),
		MessageType: "Duration",
		In:          `"10s"`,
	}
	require.NoError(t, cli.Run())
	requireJSONFileContent(t, `"10s"`, cli.Out)
}

func TestFDSInput(t *testing.T) {
	tmpDir := t.TempDir()
	cli := PBConfig{
		Out:         filepath.Join(tmpDir, "out.json"),
		MessageType: "FileDescriptorSet",
		In:          "@testdata/options.pb",
	}
	require.NoError(t, cli.Run())
	requireJSONFilesEqual(t, "testdata/golden/TestFDSInput.json", cli.Out)
}

func requireJSONFileContent(t *testing.T, wantStr string, gotFile string) {
	t.Helper()
	b, err := os.ReadFile(gotFile)
	require.NoError(t, err)
	require.JSONEq(t, wantStr, string(b))
}

func requireJSONFilesEqual(t *testing.T, wantFile string, gotFile string) {
	t.Helper()
	got, err := os.ReadFile(gotFile)
	require.NoError(t, err)
	want, err := os.ReadFile(wantFile)
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(got))
}
