package ash

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
)

func TestWhitelist(t *testing.T) {
	assert.Panics(t, func() {
		Whitelist(Matrix{
			Model:      &postModel{},
			Candidates: L{accessGranted(), accessGranted()},
			Access: map[string][]string{
				"Foo": {"RW", "RW"}, // <- invalid field
			},
		})
	})

	assert.Panics(t, func() {
		Whitelist(Matrix{
			Model:      &postModel{},
			Candidates: L{accessGranted(), accessGranted()},
			Access: map[string][]string{
				"Title": {"RWX", "RW"}, // <- invalid tag
			},
		})
	})

	authorizers := Whitelist(Matrix{
		Model:      &postModel{},
		Candidates: L{conditional("foo"), conditional("bar")},
		Access: map[string][]string{
			"Title":     {"RW", "RW"},
			"Published": {"R", "RW"},
		},
	})
	assert.Len(t, authorizers, 2)

	strategy := C(&Strategy{
		All: authorizers,
	})

	ctx := &fire.Context{
		Data:           fire.Map{"key": "foo"},
		Operation:      fire.Create,
		ReadableFields: []string{"Title", "Published", "Author"},
		WritableFields: []string{"Title", "Published", "Author"},
	}

	tester.WithContext(ctx, func(ctx *fire.Context) {
		err := strategy.Handler(ctx)
		assert.NoError(t, err)

		assert.Equal(t, []string{"Title", "Published"}, ctx.ReadableFields)
		assert.Equal(t, []string{"Title"}, ctx.WritableFields)
	})

	ctx = &fire.Context{
		Data:           fire.Map{"key": "bar"},
		Operation:      fire.Create,
		ReadableFields: []string{"Title", "Published", "Author"},
		WritableFields: []string{"Title", "Published", "Author"},
	}

	tester.WithContext(ctx, func(ctx *fire.Context) {
		err := strategy.Handler(ctx)
		assert.NoError(t, err)

		assert.Equal(t, []string{"Title", "Published"}, ctx.ReadableFields)
		assert.Equal(t, []string{"Title", "Published"}, ctx.WritableFields)
	})
}

func TestWhitelistFields(t *testing.T) {
	authorizer := WhitelistFields([]string{"Foo", "Bar"}, []string{"Bar"})
	assert.NotNil(t, authorizer)

	ctx := &fire.Context{
		Operation:      fire.Create,
		ReadableFields: []string{"Foo", "Bar", "Baz"},
		WritableFields: []string{"Foo", "Bar", "Baz"},
	}

	tester.WithContext(ctx, func(ctx *fire.Context) {
		enforcers, err := authorizer.Handler(ctx)
		assert.NoError(t, err)
		assert.Len(t, enforcers, 3)

		for _, enforcer := range enforcers {
			err = enforcer.Handler(ctx)
			assert.NoError(t, err)
		}

		assert.Equal(t, []string{"Foo", "Bar"}, ctx.ReadableFields)
		assert.Equal(t, []string{"Bar"}, ctx.WritableFields)
	})
}