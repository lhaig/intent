package checker

import (
	"strings"

	"github.com/lhaig/intent/internal/ast"
)

// Type represents a type in the Intent type system
type Type struct {
	Name       string // "Int", "Float", "String", "Bool", "Void", "Fn", entity name, or enum name
	IsEntity   bool
	Entity     *EntityInfo // non-nil if IsEntity
	IsEnum     bool
	EnumInfo   *EnumInfo // non-nil if IsEnum
	IsGeneric  bool      // true if TypeParams is non-empty
	TypeParams []*Type   // e.g., [TypeInt] for Array<Int>
	IsFunction bool      // true for function types (Fn)
	FnParams   []*Type   // parameter types for function types
	FnReturn   *Type     // return type for function types
}

// EntityInfo holds information about an entity type
type EntityInfo struct {
	Name           string
	Fields         map[string]*Type
	FieldOrder     []string // preserve declaration order
	HasInvariant   bool
	Methods        map[string]*MethodInfo
	HasConstructor bool
}

// MethodInfo holds information about a method
type MethodInfo struct {
	Name        string
	Params      []ParamInfo
	ReturnType  *Type
	HasRequires bool
	HasEnsures  bool
}

// ParamInfo holds information about a parameter
type ParamInfo struct {
	Name string
	Type *Type
}

// TraitInfo holds information about a trait type
type TraitInfo struct {
	Name    string
	Methods map[string]*MethodInfo
}

// EnumInfo holds information about an enum type
type EnumInfo struct {
	Name     string
	Variants []*EnumVariantInfo
}

// EnumVariantInfo holds information about an enum variant
type EnumVariantInfo struct {
	Name   string
	Fields []ParamInfo // empty for unit variants
}

// Builtin types
var (
	TypeInt    = &Type{Name: "Int"}
	TypeFloat  = &Type{Name: "Float"}
	TypeString = &Type{Name: "String"}
	TypeBool   = &Type{Name: "Bool"}
	TypeVoid   = &Type{Name: "Void"}
)

// ResolveType resolves a type reference to a Type object
func ResolveType(ref *ast.TypeRef, entities map[string]*EntityInfo, enums map[string]*EnumInfo) *Type {
	if ref == nil {
		return nil
	}
	switch ref.Name {
	case "Int":
		return TypeInt
	case "Float":
		return TypeFloat
	case "String":
		return TypeString
	case "Bool":
		return TypeBool
	case "Void":
		return TypeVoid
	case "Array":
		// Array requires exactly 1 type argument
		if len(ref.TypeArgs) != 1 {
			return nil // caller should emit error
		}
		elemType := ResolveType(ref.TypeArgs[0], entities, enums)
		if elemType == nil {
			return nil
		}
		return &Type{
			Name:       "Array",
			IsGeneric:  true,
			TypeParams: []*Type{elemType},
		}
	case "Result":
		// Result requires exactly 2 type arguments (T, E)
		if len(ref.TypeArgs) != 2 {
			return nil // caller should emit error
		}
		okType := ResolveType(ref.TypeArgs[0], entities, enums)
		errType := ResolveType(ref.TypeArgs[1], entities, enums)
		if okType == nil || errType == nil {
			return nil
		}
		return &Type{
			Name:       "Result",
			IsEnum:     true,
			IsGeneric:  true,
			TypeParams: []*Type{okType, errType},
			EnumInfo:   instantiateResult(okType, errType),
		}
	case "Option":
		// Option requires exactly 1 type argument (T)
		if len(ref.TypeArgs) != 1 {
			return nil // caller should emit error
		}
		someType := ResolveType(ref.TypeArgs[0], entities, enums)
		if someType == nil {
			return nil
		}
		return &Type{
			Name:       "Option",
			IsEnum:     true,
			IsGeneric:  true,
			TypeParams: []*Type{someType},
			EnumInfo:   instantiateOption(someType),
		}
	case "Fn":
		// Fn type: Fn(ParamTypes) -> ReturnType
		var paramTypes []*Type
		for _, pt := range ref.ParamTypes {
			resolved := ResolveType(pt, entities, enums)
			if resolved == nil {
				return nil
			}
			paramTypes = append(paramTypes, resolved)
		}
		var returnType *Type
		if ref.ReturnType != nil {
			returnType = ResolveType(ref.ReturnType, entities, enums)
			if returnType == nil {
				return nil
			}
		} else {
			returnType = TypeVoid
		}
		return &Type{
			Name:       "Fn",
			IsFunction: true,
			FnParams:   paramTypes,
			FnReturn:   returnType,
		}
	case "Map":
		// Map requires exactly 2 type arguments (K, V)
		if len(ref.TypeArgs) != 2 {
			return nil // caller should emit error
		}
		keyType := ResolveType(ref.TypeArgs[0], entities, enums)
		valType := ResolveType(ref.TypeArgs[1], entities, enums)
		if keyType == nil || valType == nil {
			return nil
		}
		// Reject unhashable key types: Float, Array, Map
		if keyType.Name == "Float" || keyType.Name == "Array" || keyType.Name == "Map" {
			return nil
		}
		return &Type{
			Name:       "Map",
			IsGeneric:  true,
			TypeParams: []*Type{keyType, valType},
		}
	default:
		// Check if it's an entity type
		if entity, ok := entities[ref.Name]; ok {
			return &Type{
				Name:     ref.Name,
				IsEntity: true,
				Entity:   entity,
			}
		}
		// Check if it's an enum type
		if enumInfo, ok := enums[ref.Name]; ok {
			return &Type{
				Name:     ref.Name,
				IsEnum:   true,
				EnumInfo: enumInfo,
			}
		}
		return nil // Unknown type
	}
}

// Equal checks if two types are equal
func (t *Type) Equal(other *Type) bool {
	if t == nil || other == nil {
		return t == other
	}
	if t.Name != other.Name {
		return false
	}
	if t.IsFunction != other.IsFunction {
		return false
	}
	if t.IsFunction {
		if len(t.FnParams) != len(other.FnParams) {
			return false
		}
		for i := range t.FnParams {
			if !t.FnParams[i].Equal(other.FnParams[i]) {
				return false
			}
		}
		return t.FnReturn.Equal(other.FnReturn)
	}
	if t.IsGeneric != other.IsGeneric {
		return false
	}
	if t.IsGeneric {
		if len(t.TypeParams) != len(other.TypeParams) {
			return false
		}
		for i := range t.TypeParams {
			if !t.TypeParams[i].Equal(other.TypeParams[i]) {
				return false
			}
		}
	}
	return true
}

// String returns the string representation of the type
func (t *Type) String() string {
	if t == nil {
		return "<nil>"
	}
	if t.IsFunction {
		params := make([]string, len(t.FnParams))
		for i, p := range t.FnParams {
			params[i] = p.String()
		}
		return "Fn(" + strings.Join(params, ", ") + ") -> " + t.FnReturn.String()
	}
	if t.IsGeneric && len(t.TypeParams) > 0 {
		params := make([]string, len(t.TypeParams))
		for i, p := range t.TypeParams {
			params[i] = p.String()
		}
		return t.Name + "<" + strings.Join(params, ", ") + ">"
	}
	return t.Name
}

// instantiateResult creates an EnumInfo for Result<T, E>
func instantiateResult(okType, errType *Type) *EnumInfo {
	return &EnumInfo{
		Name: "Result",
		Variants: []*EnumVariantInfo{
			{
				Name: "Ok",
				Fields: []ParamInfo{
					{Name: "value", Type: okType},
				},
			},
			{
				Name: "Err",
				Fields: []ParamInfo{
					{Name: "error", Type: errType},
				},
			},
		},
	}
}

// instantiateOption creates an EnumInfo for Option<T>
func instantiateOption(someType *Type) *EnumInfo {
	return &EnumInfo{
		Name: "Option",
		Variants: []*EnumVariantInfo{
			{
				Name: "Some",
				Fields: []ParamInfo{
					{Name: "value", Type: someType},
				},
			},
			{
				Name:   "None",
				Fields: []ParamInfo{}, // unit variant
			},
		},
	}
}
