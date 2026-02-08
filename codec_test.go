package rap

import (
	"errors"
	"reflect"
	"testing"

	"github.com/woozymasta/rvcfg"
)

func TestEncodeDecodeRoundTrip_MinimalAST(t *testing.T) {
	t.Parallel()

	in := rvcfg.File{
		Statements: []rvcfg.Statement{
			{
				Kind: rvcfg.NodeClass,
				Class: &rvcfg.ClassDecl{
					Name: "CfgVehicles",
					Body: []rvcfg.Statement{
						{
							Kind: rvcfg.NodeProperty,
							Property: &rvcfg.PropertyAssign{
								Name: "displayName",
								Value: rvcfg.Value{
									Kind: rvcfg.ValueScalar,
									Raw:  `"Demo"`,
								},
							},
						},
						{
							Kind: rvcfg.NodeProperty,
							Property: &rvcfg.PropertyAssign{
								Name: "armor",
								Value: rvcfg.Value{
									Kind: rvcfg.ValueScalar,
									Raw:  "1.5",
								},
							},
						},
						{
							Kind: rvcfg.NodeArrayAssign,
							ArrayAssign: &rvcfg.ArrayAssign{
								Name: "types",
								Value: rvcfg.Value{
									Kind: rvcfg.ValueArray,
									Elements: []rvcfg.Value{
										{Kind: rvcfg.ValueScalar, Raw: `"a"`},
										{Kind: rvcfg.ValueScalar, Raw: "1"},
										{Kind: rvcfg.ValueArray, Elements: []rvcfg.Value{
											{Kind: rvcfg.ValueScalar, Raw: `"nested"`},
										}},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	encoded, err := EncodeAST(in, EncodeOptions{})
	if err != nil {
		t.Fatalf("EncodeAST() error: %v", err)
	}

	if len(encoded) == 0 {
		t.Fatal("expected non-empty encoded bytes")
	}

	out, err := DecodeToAST(encoded, DecodeOptions{})
	if err != nil {
		t.Fatalf("DecodeToAST() error: %v", err)
	}

	if !reflect.DeepEqual(out.Statements, in.Statements) {
		t.Fatalf("roundtrip mismatch:\n got=%#v\nwant=%#v", out.Statements, in.Statements)
	}
}

func TestDecodeToAST_InvalidSignature(t *testing.T) {
	t.Parallel()

	_, err := DecodeToAST([]byte("not-rap"), DecodeOptions{})
	if !errors.Is(err, ErrInvalidRAP) {
		t.Fatalf("expected ErrInvalidRAP, got=%v", err)
	}
}

func TestEncodeAST_BareIdentifierUsesStringSubtype(t *testing.T) {
	t.Parallel()

	input := rvcfg.File{
		Statements: []rvcfg.Statement{
			{
				Kind: rvcfg.NodeProperty,
				Property: &rvcfg.PropertyAssign{
					Name: "x",
					Value: rvcfg.Value{
						Kind: rvcfg.ValueScalar,
						Raw:  "foo",
					},
				},
			},
		},
	}

	data, err := EncodeAST(input, EncodeOptions{})
	if err != nil {
		t.Fatalf("EncodeAST() error: %v", err)
	}

	reader := newBinaryReader(data)
	for i := 0; i < 16; i++ {
		if _, readErr := reader.readByte(); readErr != nil {
			t.Fatalf("header read failed: %v", readErr)
		}
	}

	base, err := reader.readCString()
	if err != nil {
		t.Fatalf("read root base: %v", err)
	}

	if base != "" {
		t.Fatalf("expected empty root base, got=%q", base)
	}

	count, err := reader.readCompressedInt()
	if err != nil {
		t.Fatalf("read root entry count: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected one root entry, got=%d", count)
	}

	entryType, err := reader.readByte()
	if err != nil {
		t.Fatalf("read entry type: %v", err)
	}

	if entryType != 1 {
		t.Fatalf("expected scalar entry type=1, got=%d", entryType)
	}

	subType, err := reader.readByte()
	if err != nil {
		t.Fatalf("read scalar subtype: %v", err)
	}

	if subType != 0 {
		t.Fatalf("expected string scalar subtype=0 for bare identifier, got=%d", subType)
	}
}

func TestDecodeToAST_GeneratedFixtures(t *testing.T) {
	t.Parallel()

	paths := []string{
		testDataPath("cases", "basic", "config.cpp"),
		testDataPath("cases", "byteparity", "class_with_array.cpp"),
	}

	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			parsed, err := ParseSourceFile(path, RecommendedSourceParseOptions())
			if err != nil {
				t.Fatalf("ParseSourceFile(%s) error: %v", path, err)
			}

			data, err := EncodeAST(parsed.Processed.Parse.File, EncodeOptions{})
			if err != nil {
				t.Fatalf("EncodeAST(%s) error: %v", path, err)
			}

			got, err := DecodeToAST(data, DecodeOptions{})
			if err != nil {
				t.Fatalf("DecodeToAST(%s) error: %v", path, err)
			}

			if len(got.Statements) == 0 {
				t.Fatalf("DecodeToAST(%s) produced empty root statements", path)
			}
		})
	}
}
