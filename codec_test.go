package rap

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/woozymasta/rvcfg"
)

const (
	brokenEnumOffsetBinPath  = "bin/broken_enum_offset.rvmat"
	brokenEnumOffsetTextPath = "broken_enum_offset.rvmat"
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
		testDataPath("cases", "basic", "int64_append.cpp"),
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

func TestDecodeToAST_BinaryFixture_AppendFlagsAndInt64(t *testing.T) {
	t.Parallel()

	path := testDataPath("cases", "basic", "bin", "append_i64_flags_first.bin")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error: %v", path, err)
	}

	got, err := DecodeToAST(data, DecodeOptions{})
	if err != nil {
		t.Fatalf("DecodeToAST(%s) error: %v", path, err)
	}

	if len(got.Statements) != 2 {
		t.Fatalf("expected two root statements, got=%d", len(got.Statements))
	}

	appendStmt := got.Statements[0]
	if appendStmt.Kind != rvcfg.NodeArrayAssign || appendStmt.ArrayAssign == nil {
		t.Fatalf("expected first statement as array append, got kind=%v", appendStmt.Kind)
	}

	if !appendStmt.ArrayAssign.Append {
		t.Fatal("expected first statement with append mode")
	}

	if appendStmt.ArrayAssign.Name != "voices" {
		t.Fatalf("expected append target voices, got=%q", appendStmt.ArrayAssign.Name)
	}

	if appendStmt.ArrayAssign.Value.Kind != rvcfg.ValueArray {
		t.Fatalf("expected append value array kind, got=%v", appendStmt.ArrayAssign.Value.Kind)
	}

	if len(appendStmt.ArrayAssign.Value.Elements) != 1 {
		t.Fatalf("expected one append array element, got=%d", len(appendStmt.ArrayAssign.Value.Elements))
	}

	if appendStmt.ArrayAssign.Value.Elements[0].Raw != `"Male01"` {
		t.Fatalf("expected append element \"Male01\", got=%q", appendStmt.ArrayAssign.Value.Elements[0].Raw)
	}

	propStmt := got.Statements[1]
	if propStmt.Kind != rvcfg.NodeProperty || propStmt.Property == nil {
		t.Fatalf("expected second statement as property, got kind=%v", propStmt.Kind)
	}

	if propStmt.Property.Name != "speed" {
		t.Fatalf("expected property name speed, got=%q", propStmt.Property.Name)
	}

	if propStmt.Property.Value.Raw != "10000000000" {
		t.Fatalf("expected int64 scalar raw 10000000000, got=%q", propStmt.Property.Value.Raw)
	}
}

func TestDecodeToAST_FixtureWithBrokenEnumOffset(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(testDataPath("cases", "compat", brokenEnumOffsetBinPath))
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error: %v", brokenEnumOffsetBinPath, err)
	}

	file, err := DecodeToAST(data, DecodeOptions{})
	if err != nil {
		t.Fatalf("DecodeToAST(%s) error: %v", brokenEnumOffsetBinPath, err)
	}

	if len(file.Statements) != 1 {
		t.Fatalf("expected one statement, got=%d", len(file.Statements))
	}

	statement := file.Statements[0]
	if statement.Kind != rvcfg.NodeProperty || statement.Property == nil {
		t.Fatalf("expected property statement, got kind=%v", statement.Kind)
	}

	if statement.Property.Name != "specularPower" {
		t.Fatalf("expected property name specularPower, got=%q", statement.Property.Name)
	}

	if statement.Property.Value.Raw != "0" {
		t.Fatalf("expected property value raw=0, got=%q", statement.Property.Value.Raw)
	}
}

func TestDecodeToText_FixtureWithBrokenEnumOffset(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(testDataPath("cases", "compat", brokenEnumOffsetBinPath))
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error: %v", brokenEnumOffsetBinPath, err)
	}

	text, err := DecodeToText(data, DecodeOptions{}, RenderOptions{})
	if err != nil {
		t.Fatalf("DecodeToText(%s) error: %v", brokenEnumOffsetBinPath, err)
	}

	expectedTextBytes, err := os.ReadFile(testDataPath("cases", "compat", brokenEnumOffsetTextPath))
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error: %v", brokenEnumOffsetTextPath, err)
	}

	got := strings.TrimSpace(string(text))
	expected := strings.TrimSpace(string(expectedTextBytes))
	normalize := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace
	if normalize(got) != normalize(expected) {
		t.Fatalf("decoded text mismatch:\n got=%q\nwant=%q", got, expected)
	}
}

func TestEncodeAST_ArrayAppendWireOrder(t *testing.T) {
	t.Parallel()

	input := rvcfg.File{
		Statements: []rvcfg.Statement{
			{
				Kind: rvcfg.NodeArrayAssign,
				ArrayAssign: &rvcfg.ArrayAssign{
					Name:   "voices",
					Append: true,
					Value: rvcfg.Value{
						Kind: rvcfg.ValueArray,
						Elements: []rvcfg.Value{
							{Kind: rvcfg.ValueScalar, Raw: `"Male01"`},
						},
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

	if _, err := reader.readCString(); err != nil {
		t.Fatalf("read root base: %v", err)
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

	if entryType != 5 {
		t.Fatalf("expected append entry type=5, got=%d", entryType)
	}

	flags, err := reader.readU32()
	if err != nil {
		t.Fatalf("read append flags: %v", err)
	}

	if flags != 1 {
		t.Fatalf("expected append flags=1, got=%d", flags)
	}

	name, err := reader.readCString()
	if err != nil {
		t.Fatalf("read property name: %v", err)
	}

	if name != "voices" {
		t.Fatalf("expected append property name=voices, got=%q", name)
	}
}

func TestEncodeDecode_Int64ScalarSubtype(t *testing.T) {
	t.Parallel()

	input := rvcfg.File{
		Statements: []rvcfg.Statement{
			{
				Kind: rvcfg.NodeProperty,
				Property: &rvcfg.PropertyAssign{
					Name: "speed",
					Value: rvcfg.Value{
						Kind: rvcfg.ValueScalar,
						Raw:  "10000000000",
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

	if _, err := reader.readCString(); err != nil {
		t.Fatalf("read root base: %v", err)
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

	if subType != 6 {
		t.Fatalf("expected scalar subtype=6 for int64, got=%d", subType)
	}

	name, err := reader.readCString()
	if err != nil {
		t.Fatalf("read property name: %v", err)
	}

	if name != "speed" {
		t.Fatalf("expected property name=speed, got=%q", name)
	}

	value, err := reader.readI64()
	if err != nil {
		t.Fatalf("read i64 value: %v", err)
	}

	if value != 10000000000 {
		t.Fatalf("expected int64 payload=10000000000, got=%d", value)
	}

	decoded, err := DecodeToAST(data, DecodeOptions{})
	if err != nil {
		t.Fatalf("DecodeToAST() error: %v", err)
	}

	if len(decoded.Statements) != 1 {
		t.Fatalf("expected one decoded statement, got=%d", len(decoded.Statements))
	}

	got := decoded.Statements[0]
	if got.Property == nil {
		t.Fatal("expected decoded property statement")
	}

	if got.Property.Value.Raw != "10000000000" {
		t.Fatalf("expected decoded raw int64 scalar, got=%q", got.Property.Value.Raw)
	}
}
