// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/rap

package rap

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/woozymasta/rvcfg"
)

// encodeContext stores mutable RAP encode state.
type encodeContext struct {
	writer             *binaryWriter
	enumOffsetRefPatch []int
	tailForwardLinks   []tailForwardLink
	resolvedTailPatch  []int
	lastRaw            string
	lastClass          cachedScalarClass
	lastValid          bool
}

// tailForwardLink links tail patch that should mirror target patch final value.
type tailForwardLink struct {
	fromPatchAt int
	toPatchAt   int
}

// classOffsetPatch stores class body pointer patch point.
type classOffsetPatch struct {
	class   rvcfg.ClassDecl
	patchAt int
}

// cachedScalarClass stores memoized scalar classification result.
type cachedScalarClass struct {
	data    scalarEncoding
	subType byte
}

// encodeFile encodes rvcfg AST into RAP bytes.
func encodeFile(file rvcfg.File, opts EncodeOptions) ([]byte, error) {
	preparedFile, enums, err := prepareEncodeInput(file, opts)
	if err != nil {
		return nil, err
	}

	ctx := &encodeContext{
		writer:             newBinaryWriterWithCapacity(estimateFileBinarySize(preparedFile, enums)),
		enumOffsetRefPatch: make([]int, 0, 32),
		tailForwardLinks:   make([]tailForwardLink, 0, 32),
		resolvedTailPatch:  make([]int, 0, 32),
	}

	ctx.writer.buf = append(ctx.writer.buf, rapSignature[:]...)
	ctx.writer.writeU32(0)
	ctx.writer.writeU32(8)

	enumOffsetPatch := ctx.writer.pos()
	ctx.writer.writeU32(0)

	rootTailPatch, err := ctx.encodeClassBody("", preparedFile.Statements)
	if err != nil {
		return nil, err
	}

	enumCountPos := ctx.writer.pos()
	if err := ctx.writer.patchU32Int(enumOffsetPatch, enumCountPos); err != nil {
		return nil, err
	}

	enumCountU32, err := intToU32(len(enums))
	if err != nil {
		return nil, err
	}

	sort.Ints(ctx.resolvedTailPatch)

	for _, patchAt := range ctx.enumOffsetRefPatch {
		if isResolvedPatch(ctx.resolvedTailPatch, patchAt) {
			continue
		}

		if err := ctx.writer.patchU32Int(patchAt, enumCountPos); err != nil {
			return nil, err
		}
	}

	for _, link := range ctx.tailForwardLinks {
		target, err := readU32At(ctx.writer.buf, link.toPatchAt)
		if err != nil {
			return nil, err
		}

		targetInt, err := u32ToInt(target)
		if err != nil {
			return nil, err
		}

		if err := ctx.writer.patchU32Int(link.fromPatchAt, targetInt); err != nil {
			return nil, err
		}
	}

	// Root tail must always resolve to enum section start.
	if rootTailPatch >= 0 {
		if err := ctx.patchTailTarget(rootTailPatch, enumCountPos); err != nil {
			return nil, err
		}
	}

	// BI-compatible enum footer shape:
	// u32 nEnums, then entries.
	ctx.writer.writeU32(enumCountU32)
	for _, item := range enums {
		ctx.writer.writeCString(item.Name)
		ctx.writer.writeI32(item.Value)
	}

	return ctx.writer.bytes(), nil
}

// estimateFileBinarySize approximates final RAP byte size for fewer writer reallocations.
func estimateFileBinarySize(file rvcfg.File, enums []EnumEntry) int {
	// Signature + 3 fixed u32 fields.
	size := 16
	size += estimateClassBodyBinarySize("", file.Statements)
	// Enum footer:
	// u32 OffsetToEnums + u32 nEnums + entries.
	size += 8
	for _, item := range enums {
		size += len(item.Name) + 1 + 4
	}

	if size < 1024 {
		return 1024
	}

	return size
}

// estimateClassBodyBinarySize estimates encoded bytes for class body payload.
func estimateClassBodyBinarySize(base string, statements []rvcfg.Statement) int {
	size := len(base) + 1
	size += compressedIntBinarySize(len(statements))

	useClassTailSentinel := hasClassOnlyBody(statements)
	for index, statement := range statements {
		switch statement.Kind {
		case rvcfg.NodeClass:
			if statement.Class == nil {
				continue
			}

			if statement.Class.Forward {
				// Forward class maps to RAP entry type=3 (extern class).
				size += 1 + len(statement.Class.Name) + 1
				continue
			}

			// Entry type + class name cstring + body offset u32.
			size += 1 + len(statement.Class.Name) + 1 + 4
			// BI layout appends extra u32 reference on last class entry in class body.
			if useClassTailSentinel && index == len(statements)-1 {
				size += 4
			}
			size += estimateClassBodyBinarySize(statement.Class.Base, statement.Class.Body)

		case rvcfg.NodeProperty:
			if statement.Property == nil {
				continue
			}

			// Entry type + scalar subtype + name cstring + scalar payload.
			size += 1 + 1 + len(statement.Property.Name) + 1 + estimateScalarBinarySize(statement.Property.Value)

		case rvcfg.NodeArrayAssign:
			if statement.ArrayAssign == nil {
				continue
			}

			// Entry type + optional flags u32 + name cstring + payload.
			size += 1 + len(statement.ArrayAssign.Name) + 1
			if statement.ArrayAssign.Append {
				size += 4
			}

			size += estimateArrayBinarySize(statement.ArrayAssign.Value)

		case rvcfg.NodeExtern:
			if statement.Extern == nil {
				continue
			}

			size += 1 + len(statement.Extern.Name) + 1

		case rvcfg.NodeDelete:
			if statement.Delete == nil {
				continue
			}

			size += 1 + len(statement.Delete.Name) + 1
		}
	}

	if !useClassTailSentinel {
		size += 4
	}

	return size
}

// estimateArrayBinarySize estimates encoded bytes for RAP array payload.
func estimateArrayBinarySize(value rvcfg.Value) int {
	if value.Kind != rvcfg.ValueArray {
		return 0
	}

	size := compressedIntBinarySize(len(value.Elements))
	for _, element := range value.Elements {
		if element.Kind == rvcfg.ValueArray {
			size += 1 + estimateArrayBinarySize(element)
			continue
		}

		size += estimateScalarBinarySize(element)
	}

	return size
}

// estimateScalarBinarySize estimates subtype byte + scalar wire payload.
func estimateScalarBinarySize(value rvcfg.Value) int {
	if value.Kind != rvcfg.ValueScalar {
		return 0
	}

	raw := strings.TrimSpace(value.Raw)
	if raw == "" {
		// Subtype + empty string cstring fallback.
		return 1 + 1
	}

	// Numeric wire payloads are fixed-width.
	if looksIntegerLikeRaw(raw) {
		intValue, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			if intValue >= -2147483648 && intValue <= 2147483647 {
				return 1 + 4
			}

			return 1 + 8
		}
	}

	if looksFloatRaw(raw) {
		return 1 + 4
	}

	// String-like payload as cstring. Use raw length as safe upper bound.
	return 1 + len(raw) + 1
}

// compressedIntBinarySize returns encoded byte count for RAP compressed int.
func compressedIntBinarySize(value int) int {
	if value < 0x80 {
		return 1
	}

	first := 0x80 + (value % 128)
	tailValue := ((value - first) >> 7) + 1

	return 1 + compressedIntBinarySize(tailValue)
}

// looksIntegerLikeRaw checks basic integer token syntax without full parsing.
func looksIntegerLikeRaw(raw string) bool {
	if raw == "" {
		return false
	}

	start := 0
	if raw[0] == '-' || raw[0] == '+' {
		if len(raw) == 1 {
			return false
		}

		start = 1
	}

	for i := start; i < len(raw); i++ {
		ch := raw[i]
		if ch < '0' || ch > '9' {
			return false
		}
	}

	return true
}

// encodeClassBody writes class body payload and child class bodies.
func (e *encodeContext) encodeClassBody(base string, statements []rvcfg.Statement) (int, error) {
	e.writer.writeCString(base)
	if err := e.writer.writeCompressedInt(len(statements)); err != nil {
		return -1, err
	}

	useClassTailSentinel := hasClassOnlyBody(statements)
	classPatches := make([]classOffsetPatch, 0, len(statements))
	bodyTailPatchAt := -1

	for index, statement := range statements {
		switch statement.Kind {
		case rvcfg.NodeClass:
			if statement.Class == nil {
				return -1, fmt.Errorf("%w: nil class payload", ErrInvalidRAP)
			}

			if statement.Class.Forward {
				e.writer.writeByte(3)
				e.writer.writeCString(statement.Class.Name)

				continue
			}

			e.writer.writeByte(0)
			e.writer.writeCString(statement.Class.Name)
			patchAt := e.writer.pos()
			e.writer.writeU32(0)
			if useClassTailSentinel && index == len(statements)-1 {
				bodyTailPatchAt = e.writer.pos()
				e.writer.writeU32(0)
			}

			classPatches = append(classPatches, classOffsetPatch{
				patchAt: patchAt,
				class:   *statement.Class,
			})

		case rvcfg.NodeProperty:
			if statement.Property == nil {
				return -1, fmt.Errorf("%w: nil property payload", ErrInvalidRAP)
			}

			if err := e.encodeScalarProperty(*statement.Property); err != nil {
				return -1, err
			}

		case rvcfg.NodeArrayAssign:
			if statement.ArrayAssign == nil {
				return -1, fmt.Errorf("%w: nil array payload", ErrInvalidRAP)
			}

			if err := e.encodeArrayAssign(*statement.ArrayAssign); err != nil {
				return -1, err
			}

		case rvcfg.NodeExtern:
			if statement.Extern == nil {
				return -1, fmt.Errorf("%w: nil extern payload", ErrInvalidRAP)
			}

			e.writer.writeByte(3)
			e.writer.writeCString(statement.Extern.Name)

		case rvcfg.NodeDelete:
			if statement.Delete == nil {
				return -1, fmt.Errorf("%w: nil delete payload", ErrInvalidRAP)
			}

			e.writer.writeByte(4)
			e.writer.writeCString(statement.Delete.Name)

		default:
			return -1, fmt.Errorf("%w: unsupported statement kind=%s", ErrNotImplemented, statement.Kind)
		}
	}

	if !useClassTailSentinel {
		bodyTailPatchAt = e.writer.pos()
		e.writer.writeU32(0)
	}

	if bodyTailPatchAt >= 0 {
		e.enumOffsetRefPatch = append(e.enumOffsetRefPatch, bodyTailPatchAt)
	}

	prevChildTailPatch := -1
	for _, patch := range classPatches {
		childStart := e.writer.pos()
		if err := e.writer.patchU32Int(patch.patchAt, childStart); err != nil {
			return -1, err
		}

		if prevChildTailPatch >= 0 {
			if err := e.patchTailTarget(prevChildTailPatch, childStart); err != nil {
				return -1, err
			}
		}

		childTailPatch, err := e.encodeClassBody(patch.class.Base, patch.class.Body)
		if err != nil {
			return -1, err
		}

		prevChildTailPatch = childTailPatch
	}

	if prevChildTailPatch >= 0 && bodyTailPatchAt >= 0 {
		e.tailForwardLinks = append(e.tailForwardLinks, tailForwardLink{
			fromPatchAt: prevChildTailPatch,
			toPatchAt:   bodyTailPatchAt,
		})
	}

	return bodyTailPatchAt, nil
}

// patchTailTarget writes resolved tail pointer and marks patch as resolved.
func (e *encodeContext) patchTailTarget(patchAt int, target int) error {
	if err := e.writer.patchU32Int(patchAt, target); err != nil {
		return err
	}

	e.resolvedTailPatch = append(e.resolvedTailPatch, patchAt)

	return nil
}

// isResolvedPatch reports whether sorted list contains patch offset.
func isResolvedPatch(sorted []int, patchAt int) bool {
	index := sort.SearchInts(sorted, patchAt)
	return index < len(sorted) && sorted[index] == patchAt
}

// hasClassOnlyBody reports whether all statements in body are class declarations.
func hasClassOnlyBody(statements []rvcfg.Statement) bool {
	if len(statements) == 0 {
		return false
	}

	for _, statement := range statements {
		if statement.Kind != rvcfg.NodeClass {
			return false
		}

		if statement.Class == nil || statement.Class.Forward {
			return false
		}
	}

	return true
}

// readU32At reads little-endian uint32 from absolute byte offset.
func readU32At(data []byte, at int) (uint32, error) {
	if at < 0 || at+4 > len(data) {
		return 0, fmt.Errorf("%w: read u32 out of bounds at %d", ErrInvalidRAP, at)
	}

	return binary.LittleEndian.Uint32(data[at : at+4]), nil
}

// encodeScalarProperty writes scalar statement entry type=1.
func (e *encodeContext) encodeScalarProperty(property rvcfg.PropertyAssign) error {
	subType, scalarData, err := e.classifyScalar(property.Value)
	if err != nil {
		return err
	}

	e.writer.writeByte(1)
	e.writer.writeByte(subType)
	e.writer.writeCString(property.Name)

	switch subType {
	case 0, 4: // string, variable-like
		e.writer.writeCString(scalarData.stringValue)

	case 1: // float
		e.writer.writeF32(scalarData.floatValue)

	case 2: // int32
		e.writer.writeI32(scalarData.intValue)

	case 6: // int64
		e.writer.writeI64(scalarData.int64Value)

	default:
		return fmt.Errorf("%w: unsupported scalar subtype=%d", ErrUnsupportedScalar, subType)
	}

	return nil
}

// encodeArrayAssign writes array statement entry type=2 or type=5.
func (e *encodeContext) encodeArrayAssign(assign rvcfg.ArrayAssign) error {
	entryType := byte(2)
	if assign.Append {
		entryType = 5
	}

	e.writer.writeByte(entryType)
	if assign.Append {
		e.writer.writeU32(1)
	}
	e.writer.writeCString(assign.Name)

	return e.encodeArrayValue(assign.Value)
}

// encodeArrayValue writes RAP array payload.
func (e *encodeContext) encodeArrayValue(value rvcfg.Value) error {
	if value.Kind != rvcfg.ValueArray {
		return fmt.Errorf("%w: array assignment requires array value", ErrInvalidRAP)
	}

	if err := e.writer.writeCompressedInt(len(value.Elements)); err != nil {
		return err
	}

	for _, element := range value.Elements {
		if element.Kind == rvcfg.ValueArray {
			e.writer.writeByte(3)
			if err := e.encodeArrayValue(element); err != nil {
				return err
			}

			continue
		}

		subType, scalarData, err := e.classifyScalar(element)
		if err != nil {
			return err
		}

		e.writer.writeByte(subType)
		switch subType {
		case 0, 4: // string, variable-like
			e.writer.writeCString(scalarData.stringValue)

		case 1: // float
			e.writer.writeF32(scalarData.floatValue)

		case 2: // int32
			e.writer.writeI32(scalarData.intValue)

		case 6: // int64
			e.writer.writeI64(scalarData.int64Value)

		default:
			return fmt.Errorf("%w: unsupported array scalar subtype=%d", ErrUnsupportedScalar, subType)
		}
	}

	return nil
}

// classifyScalar memoizes scalar classification by raw text.
func (e *encodeContext) classifyScalar(value rvcfg.Value) (byte, scalarEncoding, error) {
	if value.Kind != rvcfg.ValueScalar {
		return 0, scalarEncoding{}, fmt.Errorf("%w: expected scalar value", ErrUnsupportedScalar)
	}

	key := strings.TrimSpace(value.Raw)
	if key == "" {
		return 0, scalarEncoding{}, fmt.Errorf("%w: empty scalar raw", ErrUnsupportedScalar)
	}

	if e.lastValid && e.lastRaw == key {
		return e.lastClass.subType, e.lastClass.data, nil
	}

	subType, data, err := classifyScalarRawTrimmed(key)
	if err != nil {
		return 0, scalarEncoding{}, err
	}

	e.lastRaw = key
	e.lastClass = cachedScalarClass{
		subType: subType,
		data:    data,
	}
	e.lastValid = true

	return subType, data, nil
}

// scalarEncoding stores classified scalar wire value.
type scalarEncoding struct {
	stringValue string
	floatValue  float32
	intValue    int32
	int64Value  int64
}

// classifyScalarRawTrimmed maps trimmed raw scalar text to RAP subtype.
func classifyScalarRawTrimmed(raw string) (byte, scalarEncoding, error) {
	if raw == "" {
		return 0, scalarEncoding{}, fmt.Errorf("%w: empty scalar raw", ErrUnsupportedScalar)
	}

	if unquoted, ok := unquoteRVCfgString(raw); ok {
		return 0, scalarEncoding{stringValue: unquoted}, nil
	}

	// Explicit variable-like syntax can be preserved for rare legacy subtype=4 use.
	if strings.HasPrefix(raw, `@"`) && strings.HasSuffix(raw, `"`) {
		unquoted, ok := unquoteRVCfgString(strings.TrimPrefix(raw, "@"))
		if ok {
			return 4, scalarEncoding{stringValue: unquoted}, nil
		}
	}

	if intValue, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if intValue >= -2147483648 && intValue <= 2147483647 {
			return 2, scalarEncoding{intValue: int32(intValue)}, nil
		}

		return 6, scalarEncoding{int64Value: intValue}, nil
	}

	if looksFloatRaw(raw) {
		floatValue, err := strconv.ParseFloat(raw, 32)
		if err == nil {
			return 1, scalarEncoding{floatValue: float32(floatValue)}, nil
		}
	}

	if isIdentifierLike(raw) {
		// Arma/DayZ toolchain stores bare identifiers as regular string subtype.
		return 0, scalarEncoding{stringValue: raw}, nil
	}

	return 0, scalarEncoding{}, fmt.Errorf("%w: cannot classify scalar %q", ErrUnsupportedScalar, raw)
}

// looksFloatRaw checks whether raw scalar likely represents float syntax.
func looksFloatRaw(raw string) bool {
	return strings.Contains(raw, ".") || strings.Contains(raw, "e") || strings.Contains(raw, "E")
}

// isIdentifierLike checks whether scalar can be encoded as subtype=4 variable-like token.
func isIdentifierLike(raw string) bool {
	if raw == "" {
		return false
	}

	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if ch >= 'a' && ch <= 'z' {
			continue
		}

		if ch >= 'A' && ch <= 'Z' {
			continue
		}

		if ch >= '0' && ch <= '9' {
			continue
		}

		if ch == '_' || ch == '.' {
			continue
		}

		return false
	}

	return true
}
