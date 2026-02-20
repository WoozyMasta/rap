// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/rap

package rap

import (
	"fmt"
	"math"
)

// u32ToInt converts uint32 to int with bounds check.
func u32ToInt(value uint32) (int, error) {
	if uint64(value) > uint64(math.MaxInt) {
		return 0, fmt.Errorf("%w: uint32 value overflows int=%d", ErrInvalidRAP, value)
	}

	return int(value), nil
}

// intToU32 converts int to uint32 with bounds check.
func intToU32(value int) (uint32, error) {
	if value < 0 || uint64(value) > uint64(math.MaxUint32) {
		return 0, fmt.Errorf("%w: int value overflows uint32=%d", ErrInvalidRAP, value)
	}

	//nolint:gosec // guarded by explicit range check above
	return uint32(value), nil
}
