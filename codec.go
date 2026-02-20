// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/rap

package rap

import "github.com/woozymasta/rvcfg"

// EnumEntry stores one RAP enum table item.
type EnumEntry struct {
	// Name is enum symbol name.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Value is signed integer payload.
	Value int32 `json:"value,omitempty" yaml:"value,omitempty"`
}

// EncodeOptions configures RAP binary encoder.
type EncodeOptions struct {
	// Enums appends enum table entries after class bodies.
	Enums []EnumEntry `json:"enums,omitempty" yaml:"enums,omitempty"`
}

// DecodeOptions configures RAP binary decoder.
type DecodeOptions struct {
	// DisableFloatNormalization keeps decoded float scalars in verbose fixed form.
	// Default false normalizes to shortest round-trip float32 text.
	DisableFloatNormalization bool `json:"disable_float_normalization,omitempty" yaml:"disable_float_normalization,omitempty"`
}

// EncodeAST encodes parsed config AST into RAP binary payload.
func EncodeAST(file rvcfg.File, opts EncodeOptions) ([]byte, error) {
	return encodeFile(file, opts)
}

// DecodeToAST decodes RAP binary payload into config AST.
func DecodeToAST(data []byte, opts DecodeOptions) (rvcfg.File, error) {
	file, _, err := decodeFile(data, opts)
	if err != nil {
		return rvcfg.File{}, err
	}

	return file, nil
}

// DecodeToASTWithEnums decodes RAP payload and returns parsed enum table.
func DecodeToASTWithEnums(data []byte, opts DecodeOptions) (rvcfg.File, []EnumEntry, error) {
	return decodeFile(data, opts)
}
