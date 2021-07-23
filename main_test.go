package main

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGolden(t *testing.T) {
	tests := map[string]struct {
		name   string
		format string
	}{
		"001-json": {name: "001", format: "json"},
		"001-pb":   {name: "001", format: "pb"},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			in := "testdata/" + tc.name + ".proto"
			expected := "testdata/" + tc.name + "-protoc." + tc.format
			out := path.Join(t.TempDir(), tc.name+"-protog."+tc.format)
			// on failure, for file comparison uncomment
			// out = path.Join(testdata, tc.name + "-protog." + tc.format)

			c := &cli{Filename: in, Out: out, Format: tc.format}

			require.NoError(t, c.AfterApply())
			require.NoError(t, run(c))
			requireContentEq(t, expected, out, tc.format)
		})
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
