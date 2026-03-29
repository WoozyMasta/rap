package rap

import (
	"strings"
	"testing"

	"github.com/woozymasta/rvcfg"
)

func TestDecodeToText_BasicConfigBin(t *testing.T) {
	t.Parallel()

	parsed, err := ParseSourceFile(testDataPath("cases", "basic", "config.cpp"), RecommendedSourceParseOptions())
	if err != nil {
		t.Fatalf("ParseSourceFile(testdata basic config): %v", err)
	}

	data, err := EncodeAST(parsed.Processed.Parse.File, EncodeOptions{})
	if err != nil {
		t.Fatalf("EncodeAST(testdata basic config): %v", err)
	}

	text, err := DecodeToText(data, DecodeOptions{}, RenderOptions{})
	if err != nil {
		t.Fatalf("DecodeToText(generated basic config bin) error: %v", err)
	}

	if len(text) == 0 {
		t.Fatal("expected non-empty text output")
	}

	out := string(text)
	if !strings.Contains(out, "class CfgVehicles") {
		t.Fatalf("expected decoded text to contain class CfgVehicles, got:\n%s", out)
	}
}

func TestDecodeToText_ClassWithArrayBin(t *testing.T) {
	t.Parallel()

	parsed, err := ParseSourceFile(testDataPath("cases", "byteparity", "class_with_array.cpp"), RecommendedSourceParseOptions())
	if err != nil {
		t.Fatalf("ParseSourceFile(testdata class_with_array): %v", err)
	}

	data, err := EncodeAST(parsed.Processed.Parse.File, EncodeOptions{})
	if err != nil {
		t.Fatalf("EncodeAST(testdata class_with_array): %v", err)
	}

	text, err := DecodeToText(data, DecodeOptions{}, RenderOptions{})
	if err != nil {
		t.Fatalf("DecodeToText(generated class_with_array bin) error: %v", err)
	}

	if len(text) == 0 {
		t.Fatal("expected non-empty text output")
	}

	out := string(text)
	if !strings.Contains(out, "class CfgA") {
		t.Fatalf("expected decoded text to contain class CfgA, got:\n%s", out)
	}
}

func TestDecodeToText_EmitEnumBlock(t *testing.T) {
	t.Parallel()

	file := rvcfg.File{
		Statements: []rvcfg.Statement{
			{
				Kind: rvcfg.NodeClass,
				Class: &rvcfg.ClassDecl{
					Name: "Cfg",
				},
			},
			{
				Kind: rvcfg.NodeEnum,
				Enum: &rvcfg.EnumDecl{
					Items: []rvcfg.EnumItem{
						{Name: "E_A"},
						{Name: "E_B", ValueRaw: "10"},
					},
				},
			},
		},
	}

	data, err := EncodeAST(file, EncodeOptions{})
	if err != nil {
		t.Fatalf("EncodeAST() error: %v", err)
	}

	withoutEnums, err := DecodeToText(data, DecodeOptions{}, RenderOptions{})
	if err != nil {
		t.Fatalf("DecodeToText() without enums error: %v", err)
	}

	if strings.Contains(string(withoutEnums), "enum") {
		t.Fatalf("expected no enum block by default, got:\n%s", string(withoutEnums))
	}

	withEnums, err := DecodeToText(data, DecodeOptions{}, RenderOptions{
		EmitEnumBlock: true,
	})
	if err != nil {
		t.Fatalf("DecodeToText() with enums error: %v", err)
	}

	out := string(withEnums)
	if !strings.Contains(out, "enum") || !strings.Contains(out, "E_A = 0") || !strings.Contains(out, "E_B = 10") {
		t.Fatalf("expected synthetic enum block, got:\n%s", out)
	}
}

func TestDecodeToText_BIStyleStringEscaping(t *testing.T) {
	t.Parallel()

	file := rvcfg.File{
		Statements: []rvcfg.Statement{
			{
				Kind: rvcfg.NodeProperty,
				Property: &rvcfg.PropertyAssign{
					Name: "onExecute",
					Value: rvcfg.Value{
						Kind: rvcfg.ValueScalar,
						Raw:  `"textLog ""%1"""`,
					},
				},
			},
		},
	}

	data, err := EncodeAST(file, EncodeOptions{})
	if err != nil {
		t.Fatalf("EncodeAST() error: %v", err)
	}

	text, err := DecodeToText(data, DecodeOptions{}, RenderOptions{})
	if err != nil {
		t.Fatalf("DecodeToText() error: %v", err)
	}

	if !strings.Contains(string(text), `onExecute = "textLog ""%1""";`) {
		t.Fatalf("expected BI-style string escaping, got:\n%s", string(text))
	}
}
