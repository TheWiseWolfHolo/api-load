package dialect

// RequestValidationError 只含代码定义的字段名和规则，不包含请求值。
type RequestValidationError struct {
	Field string
	Rule  string
}

func (e *RequestValidationError) Error() string { return e.Field + ": " + e.Rule }
