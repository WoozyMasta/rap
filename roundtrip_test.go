package rap

import (
	"bytes"
	"testing"

	"github.com/woozymasta/rvcfg"
)

func TestRoundTrip_RepresentativeFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		opts SourceParseOptions
	}{
		{
			name: "basic testdata config",
			path: testDataPath("cases", "basic", "config.cpp"),
			opts: RecommendedSourceParseOptions(),
		},
		{
			name: "byteparity class_with_array",
			path: testDataPath("cases", "byteparity", "class_with_array.cpp"),
			opts: SourceParseOptions{
				Preprocess: rvcfg.PreprocessOptions{
					EnableExecEvalIntrinsics: true,
				},
				Parse: rvcfg.ParseOptions{
					CaptureScalarRaw:             true,
					AutoFixMissingClassSemicolon: true,
				},
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := ParseSourceFile(test.path, test.opts)
			if err != nil {
				t.Fatalf("ParseSourceFile(%s) error: %v", test.path, err)
			}

			firstBin, err := EncodeAST(parsed.Processed.Parse.File, EncodeOptions{})
			if err != nil {
				t.Fatalf("EncodeAST(first) error: %v", err)
			}

			decoded, err := DecodeToAST(firstBin, DecodeOptions{
				DisableFloatNormalization: true,
			})
			if err != nil {
				t.Fatalf("DecodeToAST(firstBin) error: %v", err)
			}

			secondBin, err := EncodeAST(decoded, EncodeOptions{})
			if err != nil {
				t.Fatalf("EncodeAST(second) error: %v", err)
			}

			if !bytes.Equal(firstBin, secondBin) {
				t.Fatalf("binary round-trip mismatch for %s", test.path)
			}
		})
	}
}
