package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunJSON(t *testing.T) {
	tmpDir := t.TempDir()
	files, err := registryFiles("testdata/pbtest.pb")
	require.NoError(t, err)

	cfg := PBConfig{
		Protoset:    files,
		Out:         filepath.Join(tmpDir, "out.json"),
		MessageType: "BaseMessage",
		In:          `{"f": "F" }`,
	}

	formats := []string{"json", "j", ""}
	for _, format := range formats {
		t.Run("format-"+format, func(t *testing.T) {
			cfg.OutFormat = format
			require.NoError(t, run(cfg))
			want := `{"f": "F" }`
			requireJSONFileContent(t, want, cfg.Out)
		})
	}
}

func TestRunPrototext(t *testing.T) {
	tmpDir := t.TempDir()
	files, err := registryFiles("testdata/pbtest.pb")
	require.NoError(t, err)

	cfg := PBConfig{
		Protoset:    files,
		Out:         filepath.Join(tmpDir, "out.txt"),
		MessageType: "BaseMessage",
		In:          `{"f": "F" }`,
	}
	formats := []string{"txt", "t", "prototxt"}
	for _, format := range formats {
		t.Run("format-"+format, func(t *testing.T) {
			cfg.OutFormat = format
			require.NoError(t, run(cfg))
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
	files, err := registryFiles("testdata/pbtest.pb")
	require.NoError(t, err)

	cfg := PBConfig{
		Protoset: files,
		Out:      filepath.Join(tmpDir, "out.json"),
		In:       `{"f": "F" }`,
	}
	messageTypes := []string{"BaseMessage", "pbtest.BaseMessage", ".pbtest.BaseMessage", "basemessage"}
	for _, messageType := range messageTypes {
		t.Run("message-"+messageType, func(t *testing.T) {
			cfg.MessageType = messageType
			require.NoError(t, run(cfg))
			want := `{"f": "F" }`
			requireJSONFileContent(t, want, cfg.Out)
		})
	}
}

func TestRunMessageErr(t *testing.T) {
	tmpDir := t.TempDir()
	files, err := registryFiles("testdata/pbtest.pb")
	require.NoError(t, err)

	cfg := PBConfig{
		Protoset: files,
		Out:      filepath.Join(tmpDir, "out.json"),
		In:       `{"f": "F" }`,
	}
	messageTypes := []string{"Message", "..pbtest.BaseMessage"}
	for _, messageType := range messageTypes {
		t.Run("message-"+messageType, func(t *testing.T) {
			cfg.MessageType = messageType
			require.Error(t, run(cfg))
		})
	}
}

func TestRunInErr(t *testing.T) {
	tmpDir := t.TempDir()
	files, err := registryFiles("testdata/pbtest.pb")
	require.NoError(t, err)

	cfg := PBConfig{
		Protoset:    files,
		Out:         filepath.Join(tmpDir, "out.json"),
		MessageType: "BaseMessage",
		In:          `{"MISSING": "F" }`,
	}
	require.Error(t, run(cfg))
}

func requireJSONFileContent(t *testing.T, wantStr string, gotFile string) {
	t.Helper()
	b, err := os.ReadFile(gotFile)
	require.NoError(t, err)
	require.JSONEq(t, wantStr, string(b))
}
