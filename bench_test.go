package rap

import (
	"os"
	"testing"
)

// loadBenchmarkFile reads fixture bytes for benchmark setup.
func loadBenchmarkFile(b *testing.B, path string) []byte {
	b.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatalf("read %s: %v", path, err)
	}

	return data
}

// loadBenchmarkConfigAST builds AST once for encode benchmarks.
func loadBenchmarkConfigAST(b *testing.B) SourceParseResult {
	b.Helper()

	result, err := ParseSourceFile(testDataPath("cases", "basic", "config.cpp"), RecommendedSourceParseOptions())
	if err != nil {
		b.Fatalf("ParseSourceFile(testdata basic config): %v", err)
	}

	return result
}

// loadBenchmarkEncodedConfig builds RAP binary once from basic config fixture.
func loadBenchmarkEncodedConfig(b *testing.B) []byte {
	b.Helper()

	parsed := loadBenchmarkConfigAST(b)
	bin, err := EncodeAST(parsed.Processed.Parse.File, EncodeOptions{})
	if err != nil {
		b.Fatalf("EncodeAST(testdata basic config): %v", err)
	}

	return bin
}

func BenchmarkDecodeConfigBin(b *testing.B) {
	data := loadBenchmarkEncodedConfig(b)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		_, err := DecodeToAST(data, DecodeOptions{})
		if err != nil {
			b.Fatalf("DecodeToAST: %v", err)
		}
	}
}

func BenchmarkDecodeClassWithArrayBin(b *testing.B) {
	parsed, err := ParseSourceFile(testDataPath("cases", "byteparity", "class_with_array.cpp"), RecommendedSourceParseOptions())
	if err != nil {
		b.Fatalf("ParseSourceFile(byteparity class_with_array): %v", err)
	}

	data, err := EncodeAST(parsed.Processed.Parse.File, EncodeOptions{})
	if err != nil {
		b.Fatalf("EncodeAST(byteparity class_with_array): %v", err)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		_, err := DecodeToAST(data, DecodeOptions{})
		if err != nil {
			b.Fatalf("DecodeToAST: %v", err)
		}
	}
}

func BenchmarkEncodeConfigAST(b *testing.B) {
	parsed := loadBenchmarkConfigAST(b)

	encodedOnce, err := EncodeAST(parsed.Processed.Parse.File, EncodeOptions{})
	if err != nil {
		b.Fatalf("EncodeAST setup: %v", err)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(encodedOnce)))

	for i := 0; i < b.N; i++ {
		_, runErr := EncodeAST(parsed.Processed.Parse.File, EncodeOptions{})
		if runErr != nil {
			b.Fatalf("EncodeAST: %v", runErr)
		}
	}
}

func BenchmarkEncodeConfigBinDecodedAST(b *testing.B) {
	data := loadBenchmarkEncodedConfig(b)
	decoded, err := DecodeToAST(data, DecodeOptions{})
	if err != nil {
		b.Fatalf("DecodeToAST setup: %v", err)
	}

	encodedOnce, err := EncodeAST(decoded, EncodeOptions{})
	if err != nil {
		b.Fatalf("EncodeAST setup: %v", err)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(encodedOnce)))

	for i := 0; i < b.N; i++ {
		_, runErr := EncodeAST(decoded, EncodeOptions{})
		if runErr != nil {
			b.Fatalf("EncodeAST: %v", runErr)
		}
	}
}
