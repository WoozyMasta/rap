// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/rap

package rap

import "github.com/woozymasta/rvcfg"

// SourceParseOptions configures text source parse pipeline delegated to rvcfg.
type SourceParseOptions struct {
	// Preprocess configures include/macro processing.
	Preprocess rvcfg.PreprocessOptions `json:"preprocess,omitzero" yaml:"preprocess,omitempty"`

	// Parse configures parser behavior for processed text.
	Parse rvcfg.ParseOptions `json:"parse,omitzero" yaml:"parse,omitempty"`
}

// SourceParseResult stores source parse pipeline output.
type SourceParseResult struct {
	// Processed keeps preprocess+parse result from rvcfg.
	Processed rvcfg.ProcessAndParseResult `json:"processed,omitzero" yaml:"processed,omitempty"`
}

// RecommendedSourceParseOptions returns RAP-oriented defaults for source parsing.
//
// Defaults:
//   - Parse.CaptureScalarRaw = true
//   - Preprocess.EnableExecEvalIntrinsics = true
func RecommendedSourceParseOptions() SourceParseOptions {
	return SourceParseOptions{
		Preprocess: rvcfg.PreprocessOptions{
			EnableExecEvalIntrinsics: true,
		},
		Parse: rvcfg.ParseOptions{
			CaptureScalarRaw: true,
		},
	}
}

// ParseSourceFileWithDefaults runs ParseSourceFile with RecommendedSourceParseOptions.
func ParseSourceFileWithDefaults(path string) (SourceParseResult, error) {
	return ParseSourceFile(path, RecommendedSourceParseOptions())
}

// ParseSourceFile runs rvcfg processed pipeline for config-like source input.
//
// Recommended RAP-compatible options:
//   - Parse.CaptureScalarRaw = true
//   - Preprocess.EnableExecEvalIntrinsics = true
//
// Recommended "full preprocess/macro" setup for game-like sources:
//   - Preprocess.IncludeDirs with include roots
//   - Preprocess.Defines for external symbols/flags
//   - Preprocess.EnableDynamicIntrinsics = true (only if source relies on DATE/TIME/COUNTER/RAND intrinsics)
func ParseSourceFile(path string, opts SourceParseOptions) (SourceParseResult, error) {
	processed, err := rvcfg.ProcessAndParseFile(path, opts.Preprocess, opts.Parse)
	result := SourceParseResult{
		Processed: processed,
	}
	if err != nil {
		return result, err
	}

	return result, nil
}
