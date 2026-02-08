package rap

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEncodeAST_ByteParity_GoldenBins(t *testing.T) {
	t.Parallel()

	cases := []struct {
		source string
		golden string
	}{
		{
			source: testDataPath("cases", "byteparity", "class_only_3.cpp"),
			golden: testDataPath("cases", "byteparity", "bin", "class_only_3.bi.bin"),
		},
		{
			source: testDataPath("cases", "byteparity", "class_plus_prop.cpp"),
			golden: testDataPath("cases", "byteparity", "bin", "class_plus_prop.bi.bin"),
		},
		{
			source: testDataPath("cases", "byteparity", "class_chain.cpp"),
			golden: testDataPath("cases", "byteparity", "bin", "class_chain.bi.bin"),
		},
		{
			source: testDataPath("cases", "byteparity", "class_with_array.cpp"),
			golden: testDataPath("cases", "byteparity", "bin", "class_with_array.bi.bin"),
		},
	}

	for _, test := range cases {
		test := test

		t.Run(filepath.Base(test.source), func(t *testing.T) {
			t.Parallel()

			parsed, err := ParseSourceFile(test.source, RecommendedSourceParseOptions())
			if err != nil {
				t.Fatalf("ParseSourceFile(%s) error: %v", test.source, err)
			}

			encoded, err := EncodeAST(parsed.Processed.Parse.File, EncodeOptions{})
			if err != nil {
				t.Fatalf("EncodeAST(%s) error: %v", test.source, err)
			}

			golden, err := os.ReadFile(test.golden)
			if err != nil {
				t.Fatalf("read golden %s: %v", test.golden, err)
			}

			if !bytes.Equal(encoded, golden) {
				t.Fatalf("byte parity mismatch for %s: ours=%d golden=%d", test.source, len(encoded), len(golden))
			}
		})
	}
}
