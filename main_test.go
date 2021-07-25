package main

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGolden generates output for all testdata/*.proto files, comparing
// against the golden files for the specified formats.
func TestGolden(t *testing.T) {
	formats := []string{"json", "pb"}
	protos, err := filepath.Glob("testdata/*.proto")
	require.NoError(t, err)

	for _, proto := range protos {
		for _, format := range formats {
			name := strings.TrimSuffix(filepath.Base(proto), ".proto")
			testName := name + "-" + format
			t.Run(testName, func(t *testing.T) {
				in := "testdata/" + name + ".proto"
				expected := "testdata/" + name + "-protoc." + format
				out := path.Join(t.TempDir(), name+"-protog."+format)
				// on failure, for file comparison uncomment
				// out = path.Join("testdata", name+"-protog."+format)

				c := &cli{Filename: in, Out: out, Format: format}

				require.NoError(t, c.AfterApply())
				require.NoError(t, run(c))
				requireContentEq(t, expected, out, format)
			})
		}
	}
}

func requireContentEq(t *testing.T, fname1, fname2, format string) {
	t.Helper()
	f1, err := os.ReadFile(fname1)
	require.NoError(t, err)
	f2, err := os.ReadFile(fname2)
	require.NoError(t, err)
	if format == "json" {
		// protojson adds random whitespace to avoid byte-by-byte comparison
		require.JSONEq(t, string(f1), string(f2))
	} else {
		require.Equal(t, f1, f2)
	}
}
