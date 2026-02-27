// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/rap

package rap

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"github.com/woozymasta/rvcfg"
)

// decodeContext stores mutable decode state.
type decodeContext struct {
	reader                *binaryReader
	bodyMemo              map[int]decodedClassBody
	bodyBusy              map[int]struct{}
	disableFloatNormalize bool
	enumOffset            uint32
}

// decodedClassBody stores class body payload.
type decodedClassBody struct {
	base       string
	statements []rvcfg.Statement
	endOffset  int
}

// decodeFile decodes RAP bytes into rvcfg AST.
func decodeFile(data []byte, opts DecodeOptions) (rvcfg.File, []EnumEntry, error) {
	estimatedBodies := max(len(data)/512, 4)

	ctx := &decodeContext{
		reader:                newBinaryReader(data),
		bodyMemo:              make(map[int]decodedClassBody, estimatedBodies),
		bodyBusy:              make(map[int]struct{}, estimatedBodies),
		disableFloatNormalize: opts.DisableFloatNormalization,
	}

	if len(data) < 16 {
		return rvcfg.File{}, nil, fmt.Errorf("%w: file too small", ErrInvalidRAP)
	}

	sig0, err := ctx.reader.readByte()
	if err != nil {
		return rvcfg.File{}, nil, err
	}

	sig1, err := ctx.reader.readByte()
	if err != nil {
		return rvcfg.File{}, nil, err
	}

	sig2, err := ctx.reader.readByte()
	if err != nil {
		return rvcfg.File{}, nil, err
	}

	sig3, err := ctx.reader.readByte()
	if err != nil {
		return rvcfg.File{}, nil, err
	}

	if [4]byte{sig0, sig1, sig2, sig3} != rapSignature {
		return rvcfg.File{}, nil, fmt.Errorf("%w: invalid signature", ErrInvalidRAP)
	}

	always0, err := ctx.reader.readU32()
	if err != nil {
		return rvcfg.File{}, nil, err
	}

	if always0 != 0 {
		return rvcfg.File{}, nil, fmt.Errorf("%w: expected always0=0 got=%d", ErrInvalidRAP, always0)
	}

	always8, err := ctx.reader.readU32()
	if err != nil {
		return rvcfg.File{}, nil, err
	}

	if always8 != 8 {
		return rvcfg.File{}, nil, fmt.Errorf("%w: expected always8=8 got=%d", ErrInvalidRAP, always8)
	}

	offsetToEnums, err := ctx.reader.readU32()
	if err != nil {
		return rvcfg.File{}, nil, err
	}
	ctx.enumOffset = offsetToEnums

	offsetToEnumsInt, err := u32ToInt(offsetToEnums)
	if err != nil {
		return rvcfg.File{}, nil, err
	}

	var (
		root     decodedClassBody
		rootEnd  int
		fallback bool
	)

	enumFooterOffset := offsetToEnumsInt
	if offsetToEnumsInt > len(data) {
		fallback = true
		root, rootEnd, err = ctx.decodeClassBodyAtWithEnd(16)
		if err != nil {
			return rvcfg.File{}, nil, err
		}

		enumFooterOffset = rootEnd
	} else {
		root, err = ctx.decodeClassBodyAt(16)
		if err != nil {
			return rvcfg.File{}, nil, err
		}
	}

	var enums []EnumEntry
	if fallback {
		enums, err = ctx.decodeEnumFooterFallbackAt(enumFooterOffset, offsetToEnums)
	} else {
		enums, err = ctx.decodeEnumFooterStrictAt(enumFooterOffset, offsetToEnums)
	}
	if err != nil {
		return rvcfg.File{}, nil, err
	}

	return rvcfg.File{
		Statements: root.statements,
	}, enums, nil
}

// decodeEnumFooterStrictAt reads enum footer in strict mode.
func (d *decodeContext) decodeEnumFooterStrictAt(enumFooterOffset int, headerEnumOffset uint32) ([]EnumEntry, error) {
	if enumFooterOffset >= len(d.reader.data) {
		return nil, nil
	}

	if err := d.reader.seekAbsolute(enumFooterOffset); err != nil {
		return nil, err
	}

	firstFooterValue, err := d.reader.readU32()
	if err != nil {
		return nil, err
	}

	enumCount := firstFooterValue
	// BI shape keeps redundant u32 offset as first footer field.
	if firstFooterValue == headerEnumOffset {
		enumCount, err = d.reader.readU32()
		if err != nil {
			return nil, err
		}
	}

	return d.decodeEnumTable(enumCount)
}

// decodeEnumFooterFallbackAt reads enum footer for malformed header offset cases.
func (d *decodeContext) decodeEnumFooterFallbackAt(enumFooterOffset int, headerEnumOffset uint32) ([]EnumEntry, error) {
	if enumFooterOffset >= len(d.reader.data) {
		return nil, nil
	}

	if err := d.reader.seekAbsolute(enumFooterOffset); err != nil {
		return nil, err
	}

	firstFooterValue, readErr := d.reader.readU32()
	enumCount := firstFooterValue
	if readErr == nil && firstFooterValue == headerEnumOffset {
		enumCount, readErr = d.reader.readU32()
	}

	if readErr == nil {
		enums, decodeErr := d.decodeEnumTable(enumCount)
		if decodeErr == nil {
			return enums, nil
		}
	}

	return nil, nil
}

// decodeEnumTable reads enum table entries.
func (d *decodeContext) decodeEnumTable(count uint32) ([]EnumEntry, error) {
	if count == 0 {
		return nil, nil
	}

	if uint64(count) > uint64(len(d.reader.data)) {
		return nil, fmt.Errorf("%w: enum table count out of range=%d", ErrInvalidRAP, count)
	}

	out := make([]EnumEntry, 0, count)
	for range count {
		name, err := d.reader.readCString()
		if err != nil {
			return nil, err
		}

		value, err := d.reader.readI32()
		if err != nil {
			return nil, err
		}

		out = append(out, EnumEntry{
			Name:  name,
			Value: value,
		})
	}

	return out, nil
}

// decodeClassBodyAt decodes class body from absolute offset.
func (d *decodeContext) decodeClassBodyAt(offset int) (decodedClassBody, error) {
	body, _, err := d.decodeClassBodyAtInternal(offset, false)
	return body, err
}

// decodeClassBodyAtWithEnd decodes class body and reports end offset right after its payload.
func (d *decodeContext) decodeClassBodyAtWithEnd(offset int) (decodedClassBody, int, error) {
	return d.decodeClassBodyAtInternal(offset, true)
}

// decodeClassBodyAtInternal decodes class body with optional body-end capture.
func (d *decodeContext) decodeClassBodyAtInternal(offset int, needEndOffset bool) (decodedClassBody, int, error) {
	if cached, ok := d.bodyMemo[offset]; ok {
		if needEndOffset {
			return cached, cached.endOffset, nil
		}

		return cached, 0, nil
	}

	if _, busy := d.bodyBusy[offset]; busy {
		return decodedClassBody{}, 0, fmt.Errorf("%w: recursive class body offset=%d", ErrInvalidRAP, offset)
	}

	d.bodyBusy[offset] = struct{}{}
	defer delete(d.bodyBusy, offset)

	saved := d.reader.pos()
	defer func() {
		_ = d.reader.seekAbsolute(saved)
	}()

	if err := d.reader.seekAbsolute(offset); err != nil {
		return decodedClassBody{}, 0, err
	}

	base, err := d.reader.readCString()
	if err != nil {
		return decodedClassBody{}, 0, err
	}

	entryCount, err := d.reader.readCompressedInt()
	if err != nil {
		return decodedClassBody{}, 0, err
	}

	statements := make([]rvcfg.Statement, 0, entryCount)
	for i := range entryCount {
		entryType, entryErr := d.reader.readByte()
		if entryErr != nil {
			return decodedClassBody{}, 0, entryErr
		}

		switch entryType {
		case 0:
			stmt, stmtErr := d.decodeClassEntry(i == entryCount-1)
			if stmtErr != nil {
				return decodedClassBody{}, 0, stmtErr
			}

			statements = append(statements, stmt)

		case 1:
			stmt, stmtErr := d.decodeScalarEntry()
			if stmtErr != nil {
				return decodedClassBody{}, 0, stmtErr
			}

			statements = append(statements, stmt)

		case 2:
			stmt, stmtErr := d.decodeArrayEntry(false)
			if stmtErr != nil {
				return decodedClassBody{}, 0, stmtErr
			}

			statements = append(statements, stmt)

		case 3:
			name, nameErr := d.reader.readCString()
			if nameErr != nil {
				return decodedClassBody{}, 0, nameErr
			}

			statements = append(statements, rvcfg.Statement{
				Kind: rvcfg.NodeExtern,
				Extern: &rvcfg.ExternDecl{
					Name:  name,
					Class: true,
				},
			})

		case 4:
			name, nameErr := d.reader.readCString()
			if nameErr != nil {
				return decodedClassBody{}, 0, nameErr
			}

			statements = append(statements, rvcfg.Statement{
				Kind: rvcfg.NodeDelete,
				Delete: &rvcfg.DeleteStmt{
					Name: name,
				},
			})

		case 5:
			stmt, stmtErr := d.decodeArrayEntry(true)
			if stmtErr != nil {
				return decodedClassBody{}, 0, stmtErr
			}

			statements = append(statements, stmt)

		default:
			return decodedClassBody{}, 0, fmt.Errorf("%w: unsupported entry type=%d", ErrInvalidRAP, entryType)
		}
	}

	endOffset := d.reader.pos()
	out := decodedClassBody{
		base:       base,
		statements: statements,
		endOffset:  endOffset,
	}

	d.bodyMemo[offset] = out

	if !needEndOffset {
		return out, 0, nil
	}

	return out, endOffset, nil
}

// decodeClassEntry decodes class entry type=0.
func (d *decodeContext) decodeClassEntry(isLastEntry bool) (rvcfg.Statement, error) {
	name, err := d.reader.readCString()
	if err != nil {
		return rvcfg.Statement{}, err
	}

	offset, err := d.reader.readU32()
	if err != nil {
		return rvcfg.Statement{}, err
	}

	if isLastEntry {
		// BI-compatible class entry variant appends footer offset u32 for last class entry.
		// Consume it only if it matches header offset to keep compatibility with old layout.
		if d.reader.pos()+4 <= len(d.reader.data) {
			tail := binary.LittleEndian.Uint32(d.reader.data[d.reader.pos() : d.reader.pos()+4])
			if tail == d.enumOffset {
				d.reader.off += 4
			}
		}
	}

	bodyOffset, err := u32ToInt(offset)
	if err != nil {
		return rvcfg.Statement{}, err
	}

	body, err := d.decodeClassBodyAt(bodyOffset)
	if err != nil {
		return rvcfg.Statement{}, err
	}

	return rvcfg.Statement{
		Kind: rvcfg.NodeClass,
		Class: &rvcfg.ClassDecl{
			Name: name,
			Base: body.base,
			Body: body.statements,
		},
	}, nil
}

// decodeScalarEntry decodes scalar assignment entry type=1.
func (d *decodeContext) decodeScalarEntry() (rvcfg.Statement, error) {
	subType, err := d.reader.readByte()
	if err != nil {
		return rvcfg.Statement{}, err
	}

	name, err := d.reader.readCString()
	if err != nil {
		return rvcfg.Statement{}, err
	}

	raw, err := d.decodeScalarRawBySubtype(subType)
	if err != nil {
		return rvcfg.Statement{}, err
	}

	return rvcfg.Statement{
		Kind: rvcfg.NodeProperty,
		Property: &rvcfg.PropertyAssign{
			Name: name,
			Value: rvcfg.Value{
				Kind: rvcfg.ValueScalar,
				Raw:  raw,
			},
		},
	}, nil
}

// decodeArrayEntry decodes array assignment entry type=2 or type=5.
func (d *decodeContext) decodeArrayEntry(withFlags bool) (rvcfg.Statement, error) {
	var (
		name string
		err  error
	)
	appendMode := false
	if withFlags {
		// BI RAP v1 stores array append entry type=5 as:
		// flags u32, name cstring, value array.
		// Keep legacy fallback for older rap encoder outputs that used name-first order.
		saved := d.reader.pos()
		flags, flagsErr := d.reader.readU32()
		if flagsErr != nil {
			return rvcfg.Statement{}, flagsErr
		}

		if flags <= 1 {
			name, err = d.reader.readCString()
			if err != nil {
				return rvcfg.Statement{}, err
			}

			appendMode = flags&0x01 != 0
		} else {
			if seekErr := d.reader.seekAbsolute(saved); seekErr != nil {
				return rvcfg.Statement{}, seekErr
			}

			name, err = d.reader.readCString()
			if err != nil {
				return rvcfg.Statement{}, err
			}

			flags, flagsErr = d.reader.readU32()
			if flagsErr != nil {
				return rvcfg.Statement{}, flagsErr
			}

			appendMode = flags&0x01 != 0
		}
	} else {
		name, err = d.reader.readCString()
		if err != nil {
			return rvcfg.Statement{}, err
		}
	}

	value, err := d.decodeArrayValue()
	if err != nil {
		return rvcfg.Statement{}, err
	}

	return rvcfg.Statement{
		Kind: rvcfg.NodeArrayAssign,
		ArrayAssign: &rvcfg.ArrayAssign{
			Name:   name,
			Append: appendMode,
			Value:  value,
		},
	}, nil
}

// decodeArrayValue decodes RAP array payload into ValueArray.
func (d *decodeContext) decodeArrayValue() (rvcfg.Value, error) {
	count, err := d.reader.readCompressedInt()
	if err != nil {
		return rvcfg.Value{}, err
	}

	elements := make([]rvcfg.Value, 0, count)
	for range count {
		elemType, elemErr := d.reader.readByte()
		if elemErr != nil {
			return rvcfg.Value{}, elemErr
		}

		switch elemType {
		case 0, 1, 2, 4, 6:
			raw, rawErr := d.decodeScalarRawBySubtype(elemType)
			if rawErr != nil {
				return rvcfg.Value{}, rawErr
			}

			elements = append(elements, rvcfg.Value{
				Kind: rvcfg.ValueScalar,
				Raw:  raw,
			})

		case 3:
			nested, nestedErr := d.decodeArrayValue()
			if nestedErr != nil {
				return rvcfg.Value{}, nestedErr
			}

			elements = append(elements, nested)

		default:
			return rvcfg.Value{}, fmt.Errorf(
				"%w: unsupported array element subtype=%d at offset=%d",
				ErrInvalidRAP,
				elemType,
				d.reader.pos()-1,
			)
		}
	}

	return rvcfg.Value{
		Kind:     rvcfg.ValueArray,
		Elements: elements,
	}, nil
}

// decodeScalarRawBySubtype maps RAP scalar subtype to rvcfg scalar raw text.
func (d *decodeContext) decodeScalarRawBySubtype(subType byte) (string, error) {
	switch subType {
	case 0: // string
		value, err := d.reader.readCString()
		if err != nil {
			return "", err
		}

		return quoteRVCfgString(value), nil

	case 1: // float
		value, err := d.reader.readF32()
		if err != nil {
			return "", err
		}

		if d.disableFloatNormalize {
			return formatFloat32RawVerbose(value), nil
		}

		return formatFloat32RawNormalized(value), nil

	case 2: // int32
		value, err := d.reader.readI32()
		if err != nil {
			return "", err
		}

		return strconv.FormatInt(int64(value), 10), nil

	case 4: // variable-like
		value, err := d.reader.readCString()
		if err != nil {
			return "", err
		}

		// Keep output parse-friendly: represent variable-like token as string literal.
		// RAP subtype=4 is legacy/rare for Arma/DayZ corpuses.
		return quoteRVCfgString(strings.TrimSpace(value)), nil

	case 6: // int64
		value, err := d.reader.readI64()
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(value, 10), nil

	default:
		return "", fmt.Errorf("%w: unsupported scalar subtype=%d at offset=%d", ErrInvalidRAP, subType, d.reader.pos()-1)
	}
}
