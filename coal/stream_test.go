package coal

import (
	"testing"
	"time"

	"github.com/globalsign/mgo/bson"
	"github.com/stretchr/testify/assert"
)

func TestStream(t *testing.T) {
	tester.Clean()

	time.Sleep(100 * time.Millisecond)

	stream := NewStream(tester.Store, &postModel{})

	open := make(chan struct{})
	done := make(chan struct{})

	go func() {
		i := 1

		stream.Tail(func(e Event, id bson.ObjectId, m Model) {
			switch i {
			case 1:
				assert.Equal(t, Created, e)
				assert.NotZero(t, id)
				assert.NotNil(t, m)
			case 2:
				assert.Equal(t, Updated, e)
				assert.NotZero(t, id)
				assert.NotNil(t, m)
			case 3:
				assert.Equal(t, Deleted, e)
				assert.NotZero(t, id)
				assert.Nil(t, m)

				close(done)
			}

			i++
		}, func() {
			close(open)
		})
	}()

	<-open

	s := tester.Store.Copy()
	defer s.Close()

	post := Init(&postModel{
		Title: "foo",
	}).(*postModel)

	err := s.C(post).Insert(post)
	assert.NoError(t, err)

	post.Title = "bar"

	err = s.C(post).UpdateId(post.ID(), post)
	assert.NoError(t, err)

	err = s.C(post).RemoveId(post.ID())
	assert.NoError(t, err)

	<-done
}