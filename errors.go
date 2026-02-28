// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/rap

package rap

import "errors"

var (
	// ErrNotImplemented indicates codec path is declared but not implemented yet.
	ErrNotImplemented = errors.New("not implemented")

	// ErrInvalidRAP indicates malformed or unsupported RAP binary structure.
	ErrInvalidRAP = errors.New("invalid rap binary")

	// ErrUnsupportedScalar indicates scalar cannot be represented by v0 RAP scalar subtypes.
	ErrUnsupportedScalar = errors.New("unsupported scalar")

	// ErrReadRAPFile indicates failure while reading RAP payload from file path.
	ErrReadRAPFile = errors.New("read rap file failed")

	// ErrParseSource indicates failure while parsing source text before RAP encoding.
	ErrParseSource = errors.New("parse source failed")
)
