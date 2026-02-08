package rap

import (
	"reflect"
	"testing"

	"github.com/woozymasta/rvcfg"
)

func TestEncodeAST_ExtractsEnumsFromAST(t *testing.T) {
	t.Parallel()

	input := rvcfg.File{
		Statements: []rvcfg.Statement{
			{
				Kind: rvcfg.NodeClass,
				Class: &rvcfg.ClassDecl{
					Name: "CfgDemo",
					Body: []rvcfg.Statement{
						{
							Kind: rvcfg.NodeProperty,
							Property: &rvcfg.PropertyAssign{
								Name: "value",
								Value: rvcfg.Value{
									Kind: rvcfg.ValueScalar,
									Raw:  "1",
								},
							},
						},
					},
				},
			},
			{
				Kind: rvcfg.NodeEnum,
				Enum: &rvcfg.EnumDecl{
					Name: "EType",
					Items: []rvcfg.EnumItem{
						{Name: "A"},
						{Name: "B", ValueRaw: "10"},
						{Name: "C", ValueRaw: "B + 2"},
					},
				},
			},
		},
	}

	data, err := EncodeAST(input, EncodeOptions{})
	if err != nil {
		t.Fatalf("EncodeAST() error: %v", err)
	}

	decoded, enumsOut, err := DecodeToASTWithEnums(data, DecodeOptions{})
	if err != nil {
		t.Fatalf("DecodeToASTWithEnums() error: %v", err)
	}

	expected := []EnumEntry{
		{Name: "A", Value: 0},
		{Name: "B", Value: 10},
		{Name: "C", Value: 12},
	}

	if !reflect.DeepEqual(enumsOut, expected) {
		t.Fatalf("decoded enums mismatch:\n got=%#v\nwant=%#v", enumsOut, expected)
	}

	if len(decoded.Statements) != 1 || decoded.Statements[0].Kind != rvcfg.NodeClass {
		t.Fatalf("decoded root statements mismatch: %#v", decoded.Statements)
	}
}

func TestEncodeDecode_EnumTableRoundTrip(t *testing.T) {
	t.Parallel()

	input := rvcfg.File{
		Statements: []rvcfg.Statement{
			{
				Kind: rvcfg.NodeProperty,
				Property: &rvcfg.PropertyAssign{
					Name: "x",
					Value: rvcfg.Value{
						Kind: rvcfg.ValueScalar,
						Raw:  "1",
					},
				},
			},
		},
	}

	enumsIn := []EnumEntry{
		{Name: "AAA", Value: 1},
		{Name: "BBB", Value: 42},
	}

	data, err := EncodeAST(input, EncodeOptions{
		Enums: enumsIn,
	})
	if err != nil {
		t.Fatalf("EncodeAST() error: %v", err)
	}

	decoded, enumsOut, err := DecodeToASTWithEnums(data, DecodeOptions{})
	if err != nil {
		t.Fatalf("DecodeToAST() error: %v", err)
	}

	if len(decoded.Statements) != len(input.Statements) {
		t.Fatalf("decoded statements mismatch: got=%d want=%d", len(decoded.Statements), len(input.Statements))
	}

	if !reflect.DeepEqual(enumsOut, enumsIn) {
		t.Fatalf("decoded enums mismatch:\n got=%#v\nwant=%#v", enumsOut, enumsIn)
	}
}

func TestEncodeAST_EnumFooterHasBiShape(t *testing.T) {
	t.Parallel()

	data, err := EncodeAST(rvcfg.File{}, EncodeOptions{
		Enums: []EnumEntry{
			{Name: "X", Value: 7},
		},
	})
	if err != nil {
		t.Fatalf("EncodeAST() error: %v", err)
	}

	reader := newBinaryReader(data)
	for i := 0; i < 12; i++ {
		if _, readErr := reader.readByte(); readErr != nil {
			t.Fatalf("header read failed: %v", readErr)
		}
	}

	offsetToEnums, err := reader.readU32()
	if err != nil {
		t.Fatalf("read offsetToEnums failed: %v", err)
	}

	offsetToEnumsInt, err := u32ToInt(offsetToEnums)
	if err != nil {
		t.Fatalf("u32ToInt(offsetToEnums) failed: %v", err)
	}

	if err := reader.seekAbsolute(offsetToEnumsInt); err != nil {
		t.Fatalf("seek enum footer failed: %v", err)
	}

	count, err := reader.readU32()
	if err != nil {
		t.Fatalf("read enum count failed: %v", err)
	}

	if count != 1 {
		t.Fatalf("enum count mismatch: got=%d want=%d", count, 1)
	}
}

func TestEncodeAST_EnumDuplicateNameFails(t *testing.T) {
	t.Parallel()

	input := rvcfg.File{
		Statements: []rvcfg.Statement{
			{
				Kind: rvcfg.NodeEnum,
				Enum: &rvcfg.EnumDecl{
					Items: []rvcfg.EnumItem{
						{Name: "X"},
					},
				},
			},
		},
	}

	_, err := EncodeAST(input, EncodeOptions{
		Enums: []EnumEntry{
			{Name: "X", Value: 3},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate enum name error")
	}
}
