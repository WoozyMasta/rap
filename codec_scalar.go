// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/rap

package rap

import (
	"math"
	"strconv"
	"strings"
)

// quoteRVCfgString wraps source string into config-like quoted scalar.
func quoteRVCfgString(value string) string {
	escaped := strings.ReplaceAll(value, `"`, `""`)

	return `"` + escaped + `"`
}

// unquoteRVCfgString extracts config-like quoted string literal.
func unquoteRVCfgString(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) < 2 || trimmed[0] != '"' || trimmed[len(trimmed)-1] != '"' {
		return "", false
	}

	body := trimmed[1 : len(trimmed)-1]
	if body == "" {
		return "", true
	}

	var builder strings.Builder
	builder.Grow(len(body))

	for index := 0; index < len(body); index++ {
		char := body[index]

		if char == '"' {
			if index+1 < len(body) && body[index+1] == '"' {
				builder.WriteByte('"')
				index++
				continue
			}
			return "", false
		}

		// Keep legacy compatibility for older rap outputs that used \" escapes.
		// BI-style data can contain backslash before doubled quotes (\ + "").
		// In that case slash is literal and quote-pairs should be decoded by BI path.
		if char == '\\' && index+1 < len(body) && body[index+1] == '"' &&
			(index+2 >= len(body) || body[index+2] != '"') {
			builder.WriteByte('"')
			index++
			continue
		}

		builder.WriteByte(char)
	}

	return builder.String(), true
}

// formatFloat32RawVerbose formats float in verbose fixed style (CfgConvert-like).
func formatFloat32RawVerbose(value float32) string {
	text := strconv.FormatFloat(float64(value), 'f', -1, 32)
	if strings.Contains(text, ".") || strings.Contains(text, "e") || strings.Contains(text, "E") {
		return text
	}

	return text + ".0"
}

// formatFloat32RawNormalized formats float as shortest round-trip float32 scalar.
func formatFloat32RawNormalized(value float32) string {
	// Prefer human-readable decimal while keeping bounded numeric drift.
	const (
		maxPrecision = 8
		relTolerance = 1e-6
		absTolerance = 1e-6
	)

	source := float64(value)
	tolerance := math.Max(absTolerance, math.Abs(source)*relTolerance)
	best := strconv.FormatFloat(source, 'f', -1, 64)

	for precision := 0; precision <= maxPrecision; precision++ {
		candidate := strconv.FormatFloat(source, 'f', precision, 64)
		parsed, err := strconv.ParseFloat(candidate, 64)
		if err != nil {
			continue
		}

		if math.Abs(parsed-source) <= tolerance {
			best = candidate
			break
		}
	}

	if strings.Contains(best, ".") || strings.Contains(best, "e") || strings.Contains(best, "E") {
		return best
	}

	return best + ".0"
}
