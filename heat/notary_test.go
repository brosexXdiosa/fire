package heat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
)

func TestNotary(t *testing.T) {
	notary := NewNotary("test", testSecret)

	key1 := testKey{
		Base: Base{
			ID:     coal.New(),
			Expiry: time.Now().Add(time.Hour).Round(time.Second),
		},
		User: "user1234",
		Role: "admin",
	}

	token, err := notary.Issue(&key1)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	var key2 testKey
	err = notary.Verify(&key2, token)
	assert.NoError(t, err)
	key2.Expiry = key2.Expiry.Local()
	assert.Equal(t, key1, key2)
}

func TestNotaryPanics(t *testing.T) {
	assert.PanicsWithValue(t, `heat: missing name`, func() {
		NewNotary("", nil)
	})

	assert.PanicsWithValue(t, `heat: secret too small`, func() {
		NewNotary("foo", nil)
	})

	assert.NotPanics(t, func() {
		NewNotary("foo", testSecret)
	})
}
