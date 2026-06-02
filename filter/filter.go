package filter

type Operator uint

const (
	OpUndefined Operator = iota
	OpEq
	OpNEq
	OpGT
	OpGTEq
	OpLT
	OpLTEq
	OpIn
	OpNotIn
	OpLike
	OpBetween
	OpContains
	OpContainsLike
	OpIsNull
	OpNotNull
	OpNotLike // wire: "not-like" — added for ts-jsonapi parity
)

func (op Operator) Valid() bool {
	return op != OpUndefined && op <= OpNotLike
}

func (op Operator) String() string {
	switch op {
	case OpEq:
		return "=="
	case OpNEq:
		return "!="
	case OpGT:
		return ">"
	case OpGTEq:
		return ">="
	case OpLT:
		return "<"
	case OpLTEq:
		return "<="
	case OpIn:
		return "IN"
	case OpNotIn:
		return "NOT IN"
	case OpLike:
		return "LIKE"
	case OpNotLike:
		return "NOT LIKE"
	case OpBetween:
		return "BETWEEN"
	case OpContains:
		return "@>"
	case OpContainsLike:
		return "LIKE ANY"
	case OpIsNull:
		return "IS"
	case OpNotNull:
		return "IS NOT"
	case OpUndefined:
		return ""
	default:
		return ""
	}
}

type FieldFilter[T any] interface {
	Name() string
	Value() T
	Operator() Operator
}

type fieldFilter[T any] struct {
	name     string
	val      T
	operator Operator
}

func (f fieldFilter[T]) Name() string       { return f.name }
func (f fieldFilter[T]) Value() T           { return f.val }
func (f fieldFilter[T]) Operator() Operator { return f.operator }

func NewFieldFilter[T any](op Operator, name string, val T) *fieldFilter[T] {
	return &fieldFilter[T]{
		name:     name,
		val:      val,
		operator: op,
	}
}
