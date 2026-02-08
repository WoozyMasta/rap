package rap

import "errors"

var (
	// ErrNotImplemented indicates codec path is declared but not implemented yet.
	ErrNotImplemented = errors.New("not implemented")

	// ErrInvalidRAP indicates malformed or unsupported RAP binary structure.
	ErrInvalidRAP = errors.New("invalid rap binary")

	// ErrUnsupportedScalar indicates scalar cannot be represented by v0 RAP scalar subtypes.
	ErrUnsupportedScalar = errors.New("unsupported scalar")
)
