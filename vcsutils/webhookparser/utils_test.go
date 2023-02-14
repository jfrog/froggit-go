package webhookparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptional(t *testing.T) {
	t.Run("nil int pointer", func(t *testing.T) {
		var v *int = nil
		assert.Equal(t, 0, optional(v))
	})
	t.Run("non-nil int pointer", func(t *testing.T) {
		v := pointerTo(13)
		assert.Equal(t, 13, optional(v))
	})
	t.Run("nil string pointer", func(t *testing.T) {
		var v *string = nil
		assert.Equal(t, "", optional(v))
	})
	t.Run("non-nil string pointer", func(t *testing.T) {
		v := pointerTo("foo")
		assert.Equal(t, "foo", optional(v))
	})
	t.Run("nil struct pointer", func(t *testing.T) {
		var v *WebHookInfoUser = nil
		assert.Equal(t, WebHookInfoUser{}, optional(v))
	})
	t.Run("non-nil struct pointer", func(t *testing.T) {
		v := pointerTo(WebHookInfoUser{Login: "foo"})
		assert.Equal(t, WebHookInfoUser{Login: "foo"}, optional(v))
	})
}

func pointerTo[T any](t T) *T {
	return &t
}
