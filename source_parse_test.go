package rap

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/woozymasta/rvcfg"
)

func TestParseSourceFile_BasicConfig(t *testing.T) {
	t.Parallel()

	path := testDataPath("cases", "basic", "config.cpp")
	got, err := ParseSourceFile(path, RecommendedSourceParseOptions())
	if err != nil {
		t.Fatalf("ParseSourceFile(%s) error: %v", path, err)
	}

	if len(got.Processed.Parse.File.Statements) == 0 {
		t.Fatalf("expected parsed statements, got empty AST")
	}
}

func TestParseSourceFile_EncodeAST(t *testing.T) {
	t.Parallel()

	path := testDataPath("cases", "basic", "config.cpp")
	parsed, err := ParseSourceFile(path, RecommendedSourceParseOptions())
	if err != nil {
		t.Fatalf("ParseSourceFile(%s) error: %v", path, err)
	}

	encoded, err := EncodeAST(parsed.Processed.Parse.File, EncodeOptions{})
	if err != nil {
		t.Fatalf("EncodeAST(parsed ast) error: %v", err)
	}

	if len(encoded) == 0 {
		t.Fatal("expected non-empty encoded payload")
	}
}

func TestParseSourceFile_ExecEvalCompatWhenEnabled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "exec_eval.cpp")
	source := `
class CfgExecEval
{
	class Root
	{
		__EXEC(testVar = 7)
		value = __EVAL(testVar + 5);
	};
};
`

	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got, err := ParseSourceFile(path, SourceParseOptions{
		Preprocess: rvcfg.PreprocessOptions{
			EnableExecEvalIntrinsics: true,
		},
		Parse: rvcfg.ParseOptions{
			CaptureScalarRaw: true,
		},
	})
	if err != nil {
		t.Fatalf("ParseSourceFile(%s) error: %v", path, err)
	}

	root, ok := got.Processed.Parse.File.FindClass("CfgExecEval", "Root")
	if !ok || root == nil {
		t.Fatalf("expected class path CfgExecEval/Root in parsed AST")
	}

	prop, ok := root.FindProperty("value")
	if !ok || prop == nil {
		t.Fatalf("expected property value in CfgExecEval/Root")
	}

	if prop.Value.Kind != rvcfg.ValueScalar {
		t.Fatalf("expected scalar value, got %s", prop.Value.Kind)
	}

	if prop.Value.Raw != "12" {
		t.Fatalf("expected value=12 after exec/eval, got %q", prop.Value.Raw)
	}
}

func TestEncodeBytesWithDefaults(t *testing.T) {
	t.Parallel()

	source := []byte(`class CfgPatches { class TestMod { units[] = {}; }; };`)
	data, err := EncodeBytesWithDefaults("mod.cpp", source)
	if err != nil {
		t.Fatalf("EncodeBytesWithDefaults() error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("EncodeBytesWithDefaults() returned empty payload")
	}

	file, err := DecodeToAST(data, DecodeOptions{})
	if err != nil {
		t.Fatalf("DecodeToAST() error: %v", err)
	}
	if len(file.Statements) == 0 {
		t.Fatal("DecodeToAST() returned empty AST")
	}
}

func TestEncodeBytes_InvalidSource(t *testing.T) {
	t.Parallel()

	_, err := EncodeBytesWithDefaults("broken.cpp", []byte(`class Broken {`))
	if err == nil {
		t.Fatal("EncodeBytesWithDefaults() error=nil, want parse error")
	}
	if !errors.Is(err, ErrParseSource) {
		t.Fatalf("EncodeBytesWithDefaults() error=%v, want ErrParseSource", err)
	}
}
