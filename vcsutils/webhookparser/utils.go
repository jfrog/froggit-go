package webhookparser

// optional converts a nil pointer to a zero value
func optional[T any](t *T) T {
	if t == nil {
		return *new(T)
	}
	return *t
}
