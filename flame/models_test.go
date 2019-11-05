package flame

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestAddIndexes(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		i := coal.NewIndexer()
		AddTokenIndexes(i, true)
		AddApplicationIndexes(i)
		AddUserIndexes(i)

		assert.NoError(t, i.Ensure(tester.Store))
		assert.NoError(t, i.Ensure(tester.Store))
	})
}

func TestTokenInterfaces(t *testing.T) {
	var _ coal.Model = &Token{}
	var _ GenericToken = &Token{}
}

func TestApplicationInterfaces(t *testing.T) {
	coal.Init(&Application{})
	coal.Require(&Application{}, "flame-client-id")

	var _ coal.Model = &Application{}
	var _ fire.ValidatableModel = &Application{}
	var _ Client = &Application{}
}

func TestUserInterfaces(t *testing.T) {
	coal.Init(&User{})
	coal.Require(&User{}, "flame-resource-owner-id")

	var _ coal.Model = &User{}
	var _ fire.ValidatableModel = &User{}
	var _ ResourceOwner = &User{}
}

func TestApplicationValidate(t *testing.T) {
	a := coal.Init(&Application{
		Name:   "foo",
		Key:    "foo",
		Secret: "foo",
	}).(*Application)

	err := a.Validate()
	assert.NoError(t, err)
	assert.Empty(t, a.Secret)
	assert.NotEmpty(t, a.SecretHash)
}

func TestUserValidate(t *testing.T) {
	u := coal.Init(&User{
		Name:     "foo",
		Email:    "foo@example.com",
		Password: "foo",
	}).(*User)

	err := u.Validate()
	assert.NoError(t, err)
	assert.Empty(t, u.Password)
	assert.NotEmpty(t, u.PasswordHash)
}
