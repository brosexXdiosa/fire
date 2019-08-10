package glut

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
)

func TestLock(t *testing.T) {
	tester.Clean()

	// invalid token

	locked, err := Lock(tester.Store, "test", "foo", coal.Z(), time.Minute, 0)
	assert.Equal(t, "invalid token", err.Error())
	assert.False(t, locked)

	// initial lock

	token := coal.New()

	locked, err = Lock(tester.Store, "test", "foo", token, time.Minute, 0)
	assert.NoError(t, err)
	assert.True(t, locked)

	value := tester.FindLast(&Value{}).(*Value)
	assert.Equal(t, "test", value.Component)
	assert.Equal(t, "foo", value.Name)
	assert.Nil(t, value.Data)
	assert.True(t, value.Locked.After(time.Now()))
	assert.Equal(t, &token, value.Token)

	// additional lock (same token)

	locked, err = Lock(tester.Store, "test", "foo", token, time.Minute, 0)
	assert.NoError(t, err)
	assert.True(t, locked)

	value = tester.FindLast(&Value{}).(*Value)
	assert.Equal(t, "test", value.Component)
	assert.Equal(t, "foo", value.Name)
	assert.Nil(t, value.Data)
	assert.True(t, value.Locked.After(time.Now()))
	assert.Equal(t, &token, value.Token)

	// lock attempt (different token)

	locked, err = Lock(tester.Store, "test", "foo", coal.New(), time.Minute, 0)
	assert.NoError(t, err)
	assert.False(t, locked)

	value = tester.FindLast(&Value{}).(*Value)
	assert.Equal(t, "test", value.Component)
	assert.Equal(t, "foo", value.Name)
	assert.Nil(t, value.Data)
	assert.True(t, value.Locked.After(time.Now()))
	assert.Equal(t, &token, value.Token)

	// get with token

	data, loaded, err := GetLocked(tester.Store, "test", "foo", token)
	assert.NoError(t, err)
	assert.Nil(t, data)
	assert.True(t, loaded)

	// get with different token

	data, loaded, err = GetLocked(tester.Store, "test", "foo", coal.New())
	assert.NoError(t, err)
	assert.Nil(t, data)
	assert.False(t, loaded)

	// set with token

	modified, err := SetLocked(tester.Store, "test", "foo", []byte("bar"), token)
	assert.NoError(t, err)
	assert.True(t, modified)

	value = tester.FindLast(&Value{}).(*Value)
	assert.Equal(t, "test", value.Component)
	assert.Equal(t, "foo", value.Name)
	assert.Equal(t, []byte("bar"), value.Data)
	assert.True(t, value.Locked.After(time.Now()))
	assert.Equal(t, &token, value.Token)

	// set with different token

	modified, err = SetLocked(tester.Store, "test", "foo", []byte("bar"), coal.New())
	assert.NoError(t, err)
	assert.False(t, modified)

	value = tester.FindLast(&Value{}).(*Value)
	assert.Equal(t, "test", value.Component)
	assert.Equal(t, "foo", value.Name)
	assert.Equal(t, []byte("bar"), value.Data)
	assert.True(t, value.Locked.After(time.Now()))
	assert.Equal(t, &token, value.Token)

	// get with token

	data, loaded, err = GetLocked(tester.Store, "test", "foo", token)
	assert.NoError(t, err)
	assert.Equal(t, []byte("bar"), data)
	assert.True(t, loaded)

	// set non existent

	modified, err = SetLocked(tester.Store, "test", "bar", []byte("baz"), token)
	assert.NoError(t, err)
	assert.False(t, modified)

	value = tester.FindLast(&Value{}).(*Value)
	assert.Equal(t, "test", value.Component)
	assert.Equal(t, "foo", value.Name)
	assert.Equal(t, []byte("bar"), value.Data)
	assert.True(t, value.Locked.After(time.Now()))
	assert.Equal(t, &token, value.Token)

	// unlock with different token

	unlocked, err := Unlock(tester.Store, "test", "foo", coal.New(), 0)
	assert.NoError(t, err)
	assert.False(t, unlocked)

	value = tester.FindLast(&Value{}).(*Value)
	assert.Equal(t, "test", value.Component)
	assert.Equal(t, "foo", value.Name)
	assert.Equal(t, []byte("bar"), value.Data)
	assert.True(t, value.Locked.After(time.Now()))
	assert.Equal(t, &token, value.Token)

	// unlock with token

	unlocked, err = Unlock(tester.Store, "test", "foo", token, 0)
	assert.NoError(t, err)
	assert.True(t, unlocked)

	value = tester.FindLast(&Value{}).(*Value)
	assert.Equal(t, "test", value.Component)
	assert.Equal(t, "foo", value.Name)
	assert.Equal(t, []byte("bar"), value.Data)
	assert.Nil(t, value.Locked)
	assert.Nil(t, value.Token)

	// set unlocked

	modified, err = SetLocked(tester.Store, "test", "foo", []byte("baz"), coal.New())
	assert.NoError(t, err)
	assert.False(t, modified)

	value = tester.FindLast(&Value{}).(*Value)
	assert.Equal(t, "test", value.Component)
	assert.Equal(t, "foo", value.Name)
	assert.Equal(t, []byte("bar"), value.Data)
	assert.Nil(t, value.Locked)
	assert.Nil(t, value.Token)

	// lock again

	token = coal.New()

	locked, err = Lock(tester.Store, "test", "foo", token, time.Minute, 0)
	assert.NoError(t, err)
	assert.True(t, locked)

	value = tester.FindLast(&Value{}).(*Value)
	assert.Equal(t, "test", value.Component)
	assert.Equal(t, "foo", value.Name)
	assert.Equal(t, []byte("bar"), value.Data)
	assert.True(t, value.Locked.After(time.Now()))
	assert.Equal(t, &token, value.Token)

	// del with different token

	deleted, err := DelLocked(tester.Store, "test", "foo", coal.New())
	assert.NoError(t, err)
	assert.False(t, deleted)

	assert.Equal(t, 1, tester.Count(&Value{}))

	// del with token

	deleted, err = DelLocked(tester.Store, "test", "foo", token)
	assert.NoError(t, err)
	assert.True(t, deleted)

	assert.Equal(t, 0, tester.Count(&Value{}))
}