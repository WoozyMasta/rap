package rap

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/woozymasta/rvcfg"
)

// RenderOptions configures AST-to-text rendering bridge.
type RenderOptions struct {
	// Format configures final text normalization via rvcfg formatter.
	Format rvcfg.FormatOptions `json:"format,omitempty" yaml:"format,omitempty"`

	// EmitEnumBlock appends synthetic enum block reconstructed from RAP enum table.
	EmitEnumBlock bool `json:"emit_enum_block,omitempty" yaml:"emit_enum_block,omitempty"`
}

// DecodeToText decodes RAP binary and renders canonical text.
func DecodeToText(data []byte, decodeOpts DecodeOptions, renderOpts RenderOptions) ([]byte, error) {
	file, enums, err := DecodeToASTWithEnums(data, decodeOpts)
	if err != nil {
		return nil, err
	}

	if renderOpts.EmitEnumBlock && len(enums) > 0 {
		file.Statements = append(file.Statements, synthEnumStatement(enums))
	}

	raw, err := RenderAST(file)
	if err != nil {
		return nil, err
	}

	return rvcfg.FormatWithOptions(raw, renderOpts.Format)
}

// synthEnumStatement converts RAP enum table into synthetic enum statement.
func synthEnumStatement(enums []EnumEntry) rvcfg.Statement {
	items := make([]rvcfg.EnumItem, 0, len(enums))
	for _, entry := range enums {
		items = append(items, rvcfg.EnumItem{
			Name:     entry.Name,
			ValueRaw: strconv.FormatInt(int64(entry.Value), 10),
		})
	}

	return rvcfg.Statement{
		Kind: rvcfg.NodeEnum,
		Enum: &rvcfg.EnumDecl{
			Items: items,
		},
	}
}

// RenderAST renders AST into deterministic config-like text.
func RenderAST(file rvcfg.File) ([]byte, error) {
	var builder strings.Builder

	if err := writeStatements(&builder, 0, file.Statements); err != nil {
		return nil, err
	}

	return []byte(builder.String()), nil
}

// writeStatements renders statement list with indentation.
func writeStatements(builder *strings.Builder, level int, statements []rvcfg.Statement) error {
	for _, statement := range statements {
		if err := writeStatement(builder, level, statement); err != nil {
			return err
		}
	}

	return nil
}

// writeStatement renders one statement node.
func writeStatement(builder *strings.Builder, level int, statement rvcfg.Statement) error {
	switch statement.Kind {
	case rvcfg.NodeClass:
		return writeClass(builder, level, statement.Class)
	case rvcfg.NodeProperty:
		return writeProperty(builder, level, statement.Property)
	case rvcfg.NodeArrayAssign:
		return writeArrayAssign(builder, level, statement.ArrayAssign)
	case rvcfg.NodeExtern:
		return writeExtern(builder, level, statement.Extern)
	case rvcfg.NodeDelete:
		return writeDelete(builder, level, statement.Delete)
	case rvcfg.NodeEnum:
		return writeEnum(builder, level, statement.Enum)
	default:
		return fmt.Errorf("%w: unsupported statement kind=%s", ErrNotImplemented, statement.Kind)
	}
}

// writeClass renders class node.
func writeClass(builder *strings.Builder, level int, class *rvcfg.ClassDecl) error {
	if class == nil {
		return fmt.Errorf("%w: nil class node", ErrInvalidRAP)
	}

	writeIndent(builder, level)
	builder.WriteString("class ")
	builder.WriteString(class.Name)
	if class.Base != "" {
		builder.WriteString(": ")
		builder.WriteString(class.Base)
	}

	if class.Forward {
		builder.WriteString(";\n")

		return nil
	}

	builder.WriteString("\n")
	writeIndent(builder, level)
	builder.WriteString("{\n")

	if err := writeStatements(builder, level+1, class.Body); err != nil {
		return err
	}

	writeIndent(builder, level)
	builder.WriteString("};\n")

	return nil
}

// writeProperty renders scalar property assignment.
func writeProperty(builder *strings.Builder, level int, property *rvcfg.PropertyAssign) error {
	if property == nil {
		return fmt.Errorf("%w: nil property node", ErrInvalidRAP)
	}

	raw, err := renderValue(property.Value)
	if err != nil {
		return err
	}

	writeIndent(builder, level)
	builder.WriteString(property.Name)
	builder.WriteString(" = ")
	builder.WriteString(raw)
	builder.WriteString(";\n")

	return nil
}

// writeArrayAssign renders array assignment/append statement.
func writeArrayAssign(builder *strings.Builder, level int, assign *rvcfg.ArrayAssign) error {
	if assign == nil {
		return fmt.Errorf("%w: nil array assign node", ErrInvalidRAP)
	}

	raw, err := renderValue(assign.Value)
	if err != nil {
		return err
	}

	writeIndent(builder, level)
	builder.WriteString(assign.Name)
	builder.WriteString("[] ")
	if assign.Append {
		builder.WriteString("+=")
	} else {
		builder.WriteString("=")
	}

	builder.WriteString(" ")
	builder.WriteString(raw)
	builder.WriteString(";\n")

	return nil
}

// writeExtern renders extern declaration.
func writeExtern(builder *strings.Builder, level int, ext *rvcfg.ExternDecl) error {
	if ext == nil {
		return fmt.Errorf("%w: nil extern node", ErrInvalidRAP)
	}

	writeIndent(builder, level)
	if ext.Class {
		// BI CfgConvert expects forward declaration syntax in text form.
		builder.WriteString("class ")
		builder.WriteString(ext.Name)
		builder.WriteString(";\n")

		return nil
	}

	builder.WriteString("extern ")
	builder.WriteString(ext.Name)
	builder.WriteString(";\n")

	return nil
}

// writeDelete renders delete statement.
func writeDelete(builder *strings.Builder, level int, del *rvcfg.DeleteStmt) error {
	if del == nil {
		return fmt.Errorf("%w: nil delete node", ErrInvalidRAP)
	}

	writeIndent(builder, level)
	builder.WriteString("delete ")
	builder.WriteString(del.Name)
	builder.WriteString(";\n")

	return nil
}

// writeEnum renders enum declaration.
func writeEnum(builder *strings.Builder, level int, enumDecl *rvcfg.EnumDecl) error {
	if enumDecl == nil {
		return fmt.Errorf("%w: nil enum node", ErrInvalidRAP)
	}

	writeIndent(builder, level)
	builder.WriteString("enum")
	if enumDecl.Name != "" {
		builder.WriteString(" ")
		builder.WriteString(enumDecl.Name)
	}

	builder.WriteString("\n")
	writeIndent(builder, level)
	builder.WriteString("{\n")

	for _, item := range enumDecl.Items {
		writeIndent(builder, level+1)
		builder.WriteString(item.Name)
		if item.ValueRaw != "" {
			builder.WriteString(" = ")
			builder.WriteString(item.ValueRaw)
		}

		builder.WriteString(",\n")
	}

	writeIndent(builder, level)
	builder.WriteString("};\n")

	return nil
}

// renderValue renders scalar or array value.
func renderValue(value rvcfg.Value) (string, error) {
	switch value.Kind {
	case rvcfg.ValueScalar:
		if value.Raw == "" {
			return "", fmt.Errorf("%w: empty scalar raw", ErrInvalidRAP)
		}

		return value.Raw, nil

	case rvcfg.ValueArray:
		parts := make([]string, 0, len(value.Elements))
		for _, element := range value.Elements {
			text, err := renderValue(element)
			if err != nil {
				return "", err
			}

			parts = append(parts, text)
		}

		return "{" + strings.Join(parts, ", ") + "}", nil

	default:
		return "", fmt.Errorf("%w: unsupported value kind=%s", ErrNotImplemented, value.Kind)
	}
}

// writeIndent writes two-space indentation.
func writeIndent(builder *strings.Builder, level int) {
	for i := 0; i < level; i++ {
		builder.WriteString("  ")
	}
}
