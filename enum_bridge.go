// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/rap

package rap

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/woozymasta/rvcfg"
)

// prepareEncodeInput extracts enum declarations from AST and merges them with explicit options.
func prepareEncodeInput(file rvcfg.File, opts EncodeOptions) (rvcfg.File, []EnumEntry, error) {
	if !hasEnumInStatements(file.Statements) {
		if len(opts.Enums) == 0 {
			return file, nil, nil
		}

		merged := append(make([]EnumEntry, 0, len(opts.Enums)), opts.Enums...)
		if err := validateEnumTable(merged); err != nil {
			return rvcfg.File{}, nil, err
		}

		return file, merged, nil
	}

	eval := newEnumEvaluator()

	statements, astEnums, err := extractEnumsFromStatements(file.Statements, eval)
	if err != nil {
		return rvcfg.File{}, nil, err
	}

	merged := make([]EnumEntry, 0, len(astEnums)+len(opts.Enums))
	merged = append(merged, astEnums...)
	merged = append(merged, opts.Enums...)

	if err := validateEnumTable(merged); err != nil {
		return rvcfg.File{}, nil, err
	}

	out := file
	out.Statements = statements

	return out, merged, nil
}

// hasEnumInStatements reports whether statement tree contains at least one enum declaration.
func hasEnumInStatements(statements []rvcfg.Statement) bool {
	for _, statement := range statements {
		switch statement.Kind {
		case rvcfg.NodeEnum:
			return true

		case rvcfg.NodeClass:
			if statement.Class == nil {
				continue
			}

			if hasEnumInStatements(statement.Class.Body) {
				return true
			}
		}
	}

	return false
}

// extractEnumsFromStatements removes enum statements and converts them into RAP enum entries.
func extractEnumsFromStatements(statements []rvcfg.Statement, eval *enumEvaluator) ([]rvcfg.Statement, []EnumEntry, error) {
	out := make([]rvcfg.Statement, 0, len(statements))
	enums := make([]EnumEntry, 0, 8)

	for _, statement := range statements {
		switch statement.Kind {
		case rvcfg.NodeEnum:
			if statement.Enum == nil {
				return nil, nil, fmt.Errorf("%w: nil enum payload", ErrInvalidRAP)
			}

			items, err := eval.consumeDecl(*statement.Enum)
			if err != nil {
				return nil, nil, err
			}

			enums = append(enums, items...)

		case rvcfg.NodeClass:
			if statement.Class == nil {
				return nil, nil, fmt.Errorf("%w: nil class payload", ErrInvalidRAP)
			}

			classCopy := *statement.Class
			body, bodyEnums, err := extractEnumsFromStatements(classCopy.Body, eval)
			if err != nil {
				return nil, nil, err
			}

			classCopy.Body = body
			statement.Class = &classCopy
			out = append(out, statement)
			enums = append(enums, bodyEnums...)

		default:
			out = append(out, statement)
		}
	}

	return out, enums, nil
}

// validateEnumTable validates merged enum entries before writing binary footer.
func validateEnumTable(enums []EnumEntry) error {
	seen := make(map[string]struct{}, len(enums))
	for _, entry := range enums {
		if entry.Name == "" {
			return fmt.Errorf("%w: enum name is empty", ErrInvalidRAP)
		}

		if _, ok := seen[entry.Name]; ok {
			return fmt.Errorf("%w: duplicate enum name=%q", ErrInvalidRAP, entry.Name)
		}

		seen[entry.Name] = struct{}{}
	}

	return nil
}

// enumEvaluator resolves enum item values from declarations.
type enumEvaluator struct {
	symbols map[string]int64
}

// newEnumEvaluator creates enum expression evaluator.
func newEnumEvaluator() *enumEvaluator {
	return &enumEvaluator{
		symbols: make(map[string]int64, 128),
	}
}

// consumeDecl converts one enum declaration to resolved RAP entries.
func (e *enumEvaluator) consumeDecl(decl rvcfg.EnumDecl) ([]EnumEntry, error) {
	items := make([]EnumEntry, 0, len(decl.Items))
	next := int64(0)

	for _, item := range decl.Items {
		if item.Name == "" {
			return nil, fmt.Errorf("%w: enum item has empty name", ErrInvalidRAP)
		}

		value := next
		if item.ValueRaw != "" {
			parsed, err := e.eval(item.ValueRaw)
			if err != nil {
				return nil, fmt.Errorf("%w: enum %q value %q: %v", ErrInvalidRAP, item.Name, item.ValueRaw, err)
			}

			value = parsed
		}

		if value < -2147483648 || value > 2147483647 {
			return nil, fmt.Errorf("%w: enum %q value out of int32 range=%d", ErrInvalidRAP, item.Name, value)
		}

		e.symbols[item.Name] = value
		items = append(items, EnumEntry{
			Name:  item.Name,
			Value: int32(value),
		})

		next = value + 1
	}

	return items, nil
}

// eval evaluates simple integer enum expression.
func (e *enumEvaluator) eval(raw string) (int64, error) {
	parser := newEnumExprParser(raw, e.symbols)
	value, err := parser.parseExpr()
	if err != nil {
		return 0, err
	}

	parser.skipSpaces()
	if !parser.eof() {
		return 0, fmt.Errorf("unexpected token at pos=%d", parser.pos)
	}

	return value, nil
}

// enumExprParser parses integer enum expressions.
type enumExprParser struct {
	symbols map[string]int64
	input   string
	pos     int
}

// newEnumExprParser creates parser.
func newEnumExprParser(input string, symbols map[string]int64) *enumExprParser {
	return &enumExprParser{
		input:   input,
		symbols: symbols,
		pos:     0,
	}
}

// parseExpr parses bitwise-or expression.
func (p *enumExprParser) parseExpr() (int64, error) {
	left, err := p.parseBitAnd()
	if err != nil {
		return 0, err
	}

	for {
		p.skipSpaces()
		if !p.match('|') {
			return left, nil
		}

		right, rightErr := p.parseBitAnd()
		if rightErr != nil {
			return 0, rightErr
		}

		left |= right
	}
}

// parseBitAnd parses bitwise-and expression.
func (p *enumExprParser) parseBitAnd() (int64, error) {
	left, err := p.parseShift()
	if err != nil {
		return 0, err
	}

	for {
		p.skipSpaces()
		if !p.match('&') {
			return left, nil
		}

		right, rightErr := p.parseShift()
		if rightErr != nil {
			return 0, rightErr
		}

		left &= right
	}
}

// parseShift parses shift expression.
func (p *enumExprParser) parseShift() (int64, error) {
	left, err := p.parseAdd()
	if err != nil {
		return 0, err
	}

	for {
		p.skipSpaces()
		if p.matchString("<<") {
			right, rightErr := p.parseAdd()
			if rightErr != nil {
				return 0, rightErr
			}

			if right < 0 || right > 63 {
				return 0, fmt.Errorf("shift amount out of range=%d", right)
			}

			left <<= uint(right)

			continue
		}

		if p.matchString(">>") {
			right, rightErr := p.parseAdd()
			if rightErr != nil {
				return 0, rightErr
			}

			if right < 0 || right > 63 {
				return 0, fmt.Errorf("shift amount out of range=%d", right)
			}

			left >>= uint(right)

			continue
		}

		return left, nil
	}
}

// parseAdd parses + and - expression.
func (p *enumExprParser) parseAdd() (int64, error) {
	left, err := p.parseUnary()
	if err != nil {
		return 0, err
	}

	for {
		p.skipSpaces()
		if p.match('+') {
			right, rightErr := p.parseUnary()
			if rightErr != nil {
				return 0, rightErr
			}

			left += right

			continue
		}

		if p.match('-') {
			right, rightErr := p.parseUnary()
			if rightErr != nil {
				return 0, rightErr
			}

			left -= right

			continue
		}

		return left, nil
	}
}

// parseUnary parses unary + and -.
func (p *enumExprParser) parseUnary() (int64, error) {
	p.skipSpaces()
	if p.match('+') {
		return p.parseUnary()
	}

	if p.match('-') {
		value, err := p.parseUnary()
		if err != nil {
			return 0, err
		}

		return -value, nil
	}

	return p.parsePrimary()
}

// parsePrimary parses integer literal, identifier, or parenthesized expression.
func (p *enumExprParser) parsePrimary() (int64, error) {
	p.skipSpaces()
	if p.eof() {
		return 0, errors.New("expected value")
	}

	if p.match('(') {
		value, err := p.parseExpr()
		if err != nil {
			return 0, err
		}

		p.skipSpaces()
		if !p.match(')') {
			return 0, errors.New("expected ')'")
		}

		return value, nil
	}

	ch := p.peek()
	if unicode.IsDigit(ch) {
		return p.parseNumber()
	}

	if isIdentStartRune(ch) {
		name := p.parseIdent()
		value, ok := p.symbols[name]
		if !ok {
			return 0, fmt.Errorf("unknown enum symbol=%q", name)
		}

		return value, nil
	}

	return 0, fmt.Errorf("unexpected token at pos=%d", p.pos)
}

// parseNumber parses signed integer literal using Go base0 semantics.
func (p *enumExprParser) parseNumber() (int64, error) {
	start := p.pos
	for !p.eof() {
		ch := p.peek()
		if !unicode.IsDigit(ch) && (ch < 'a' || ch > 'f') && (ch < 'A' || ch > 'F') && ch != 'x' && ch != 'X' {
			break
		}

		p.pos++
	}

	raw := strings.TrimSpace(p.input[start:p.pos])
	value, err := strconv.ParseInt(raw, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number=%q", raw)
	}

	return value, nil
}

// parseIdent parses identifier token.
func (p *enumExprParser) parseIdent() string {
	start := p.pos
	p.pos++

	for !p.eof() {
		ch := p.peek()
		if !isIdentPartRune(ch) {
			break
		}

		p.pos++
	}

	return p.input[start:p.pos]
}

// skipSpaces moves parser over ASCII whitespace.
func (p *enumExprParser) skipSpaces() {
	for !p.eof() {
		switch p.input[p.pos] {
		case ' ', '\t', '\r', '\n':
			p.pos++
		default:
			return
		}
	}
}

// match matches one rune.
func (p *enumExprParser) match(ch rune) bool {
	if p.eof() || p.peek() != ch {
		return false
	}

	p.pos++

	return true
}

// matchString matches fixed token.
func (p *enumExprParser) matchString(token string) bool {
	if len(p.input)-p.pos < len(token) {
		return false
	}

	if p.input[p.pos:p.pos+len(token)] != token {
		return false
	}

	p.pos += len(token)

	return true
}

// peek returns current rune.
func (p *enumExprParser) peek() rune {
	return rune(p.input[p.pos])
}

// eof checks parser end.
func (p *enumExprParser) eof() bool {
	return p.pos >= len(p.input)
}

// isIdentStartRune reports whether rune can start identifier.
func isIdentStartRune(ch rune) bool {
	return ch == '_' || ch == '$' || unicode.IsLetter(ch)
}

// isIdentPartRune reports whether rune can continue identifier.
func isIdentPartRune(ch rune) bool {
	return isIdentStartRune(ch) || unicode.IsDigit(ch)
}
