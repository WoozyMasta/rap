package rap

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// requireCfgConvert resolves CfgConvert path or skips test.
func requireCfgConvert(t *testing.T) string {
	t.Helper()

	exe, err := resolveCfgConvertExe(".")
	if err != nil {
		t.Skipf("CfgConvert not found (%v)", err)
	}

	return exe
}

// resolveCfgConvertExe resolves CfgConvert path from env, PATH, or .env file.
func resolveCfgConvertExe(baseDir string) (string, error) {
	if exe := strings.TrimSpace(os.Getenv("CFGCONVERT_EXE")); exe != "" {
		if isExecutableFile(exe) {
			return exe, nil
		}

		return "", fmt.Errorf("CFGCONVERT_EXE points to missing file: %s", exe)
	}

	if exe, err := exec.LookPath("CfgConvert.exe"); err == nil && exe != "" {
		return exe, nil
	}

	if exe, err := exec.LookPath("CfgConvert"); err == nil && exe != "" {
		return exe, nil
	}

	if exe, ok := lookupCfgConvertFromDotEnv(baseDir); ok {
		if isExecutableFile(exe) {
			return exe, nil
		}

		return "", fmt.Errorf(".env path points to missing file: %s", exe)
	}

	return "", fmt.Errorf("not set in CFGCONVERT_EXE, not in PATH, not in .env")
}

// lookupCfgConvertFromDotEnv reads CFGCONVERT_EXE from .env.
func lookupCfgConvertFromDotEnv(baseDir string) (string, bool) {
	path := filepath.Join(baseDir, ".env")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		key, value, found := strings.Cut(trimmed, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if value == "" {
			continue
		}

		if key != "CFGCONVERT_EXE" {
			continue
		}

		if filepath.IsAbs(value) {
			return value, true
		}

		return filepath.Join(baseDir, value), true
	}

	return "", false
}

// isExecutableFile checks whether candidate path exists and is not a directory.
func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !info.IsDir()
}

// runCfgConvert executes external tool and fails test on non-zero exit.
func runCfgConvert(t *testing.T, exe string, args ...string) {
	t.Helper()

	cmd := exec.Command(exe, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CfgConvert failed: %v\nargs=%v\noutput=%s", err, args, string(output))
	}
}

func TestCfgConvert_EncodeProducedBinAccepted(t *testing.T) {
	t.Parallel()

	exe := requireCfgConvert(t)
	path := testDataPath("cases", "basic", "config.cpp")

	parsed, err := ParseSourceFile(path, RecommendedSourceParseOptions())
	if err != nil {
		t.Fatalf("ParseSourceFile(%s) error: %v", path, err)
	}

	encoded, err := EncodeAST(parsed.Processed.Parse.File, EncodeOptions{})
	if err != nil {
		t.Fatalf("EncodeAST() error: %v", err)
	}

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "out.bin")
	txtPath := filepath.Join(tmpDir, "out.cpp")

	if err := os.WriteFile(binPath, encoded, 0o600); err != nil {
		t.Fatalf("write %s: %v", binPath, err)
	}

	runCfgConvert(t, exe, "-test", binPath)
	runCfgConvert(t, exe, "-txt", "-dst", txtPath, binPath)

	text, err := os.ReadFile(txtPath)
	if err != nil {
		t.Fatalf("read %s: %v", txtPath, err)
	}

	if len(text) == 0 {
		t.Fatal("expected non-empty text produced by CfgConvert -txt")
	}
}

func TestCfgConvert_DecodeTextRebinarize(t *testing.T) {
	t.Parallel()

	exe := requireCfgConvert(t)
	parsed, err := ParseSourceFile(testDataPath("cases", "basic", "config.cpp"), RecommendedSourceParseOptions())
	if err != nil {
		t.Fatalf("ParseSourceFile(testdata basic config) error: %v", err)
	}

	data, err := EncodeAST(parsed.Processed.Parse.File, EncodeOptions{})
	if err != nil {
		t.Fatalf("EncodeAST(testdata basic config) error: %v", err)
	}

	text, err := DecodeToText(data, DecodeOptions{}, RenderOptions{})
	if err != nil {
		t.Fatalf("DecodeToText(generated basic config bin) error: %v", err)
	}

	tmpDir := t.TempDir()
	cppPath := filepath.Join(tmpDir, "decoded.cpp")
	binPath := filepath.Join(tmpDir, "rebuilt.bin")

	if err := os.WriteFile(cppPath, text, 0o600); err != nil {
		t.Fatalf("write %s: %v", cppPath, err)
	}

	runCfgConvert(t, exe, "-bin", "-dst", binPath, cppPath)
	runCfgConvert(t, exe, "-test", binPath)
}

func TestCfgConvert_ByteParity_SyntheticFixtures(t *testing.T) {
	t.Parallel()

	exe := requireCfgConvert(t)
	cases := []string{
		testDataPath("cases", "byteparity", "class_only_3.cpp"),
		testDataPath("cases", "byteparity", "class_plus_prop.cpp"),
		testDataPath("cases", "byteparity", "class_chain.cpp"),
		testDataPath("cases", "byteparity", "class_with_array.cpp"),
	}

	for _, sourcePath := range cases {
		sourcePath := sourcePath

		t.Run(filepath.Base(sourcePath), func(t *testing.T) {
			t.Parallel()

			parsed, err := ParseSourceFile(sourcePath, RecommendedSourceParseOptions())
			if err != nil {
				t.Fatalf("ParseSourceFile(%s) error: %v", sourcePath, err)
			}

			encoded, err := EncodeAST(parsed.Processed.Parse.File, EncodeOptions{})
			if err != nil {
				t.Fatalf("EncodeAST(%s) error: %v", sourcePath, err)
			}

			tmpDir := t.TempDir()
			oursBin := filepath.Join(tmpDir, "ours.bin")
			biBin := filepath.Join(tmpDir, "bi.bin")

			if err := os.WriteFile(oursBin, encoded, 0o600); err != nil {
				t.Fatalf("write ours bin: %v", err)
			}

			runCfgConvert(t, exe, "-bin", "-dst", biBin, sourcePath)

			biData, err := os.ReadFile(biBin)
			if err != nil {
				t.Fatalf("read bi bin: %v", err)
			}

			oursData, err := os.ReadFile(oursBin)
			if err != nil {
				t.Fatalf("read ours bin: %v", err)
			}

			if !bytes.Equal(oursData, biData) {
				t.Fatalf("byte parity mismatch for %s: ours=%d bi=%d", sourcePath, len(oursData), len(biData))
			}
		})
	}
}
