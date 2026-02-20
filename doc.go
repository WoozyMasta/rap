// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/rap

/*
Package rap implements RAP binary codec for DayZ/ArmA config-like data.

The package integrates with github.com/woozymasta/rvcfg for text source parsing
(preprocess + parse) and owns binary encode/decode pipeline.

Typical flow:
  - parse source text with ParseSourceFile
  - encode parsed AST with EncodeAST
  - decode RAP payload with DecodeToAST or DecodeToText

Minimal flow example:

	parsed, err := ParseSourceFileWithDefaults("config.cpp")
	if err != nil {
		// handle
	}

	bin, err := EncodeAST(parsed.Processed.Parse.File, EncodeOptions{})
	if err != nil {
		// handle
	}

	decoded, err := DecodeToAST(bin, DecodeOptions{})
	if err != nil {
		// handle
	}

	_ = decoded
*/
package rap
