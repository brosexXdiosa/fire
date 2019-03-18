package axe

import (
	"testing"
	"time"

	"github.com/globalsign/mgo/bson"
	"github.com/stretchr/testify/assert"
)

type data struct {
	Foo string `bson:"foo"`
}

func TestPool(t *testing.T) {
	tester.Clean()

	q := &Queue{Store: tester.Store}

	done := make(chan struct{})

	p := NewPool()
	p.Add(&Task{
		Name:  "foo",
		Model: &data{},
		Queue: q,
		Handler: func(m Model) (bson.M, error) {
			close(done)
			return bson.M{
				"foo": "bar",
			}, nil
		},
		Workers:     2,
		MaxAttempts: 2,
	})
	p.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue("foo", &data{Foo: "bar"}, 0)
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
	assert.Equal(t, "foo", job.Name)
	assert.Equal(t, &data{Foo: "bar"}, decodeRaw(job.Data, &data{}))
	assert.Equal(t, StatusCompleted, job.Status)
	assert.NotZero(t, job.Created)
	assert.NotZero(t, job.Delayed)
	assert.NotZero(t, job.Started)
	assert.NotZero(t, job.Ended)
	assert.NotZero(t, job.Finished)
	assert.Equal(t, 1, job.Attempts)
	assert.Equal(t, bson.M{"foo": "bar"}, job.Result)
	assert.Equal(t, "", job.Error)
	assert.Equal(t, "", job.Reason)

	p.Close()
}

func TestPoolDelayed(t *testing.T) {
	tester.Clean()

	q := &Queue{Store: tester.Store}

	done := make(chan struct{})

	p := NewPool()
	p.Add(&Task{
		Name:  "foo",
		Model: &data{},
		Queue: q,
		Handler: func(m Model) (bson.M, error) {
			close(done)
			return bson.M{
				"foo": "bar",
			}, nil
		},
		Workers:     2,
		MaxAttempts: 2,
	})
	p.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue("foo", &data{Foo: "bar"}, 100*time.Millisecond)
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
	assert.Equal(t, "foo", job.Name)
	assert.Equal(t, &data{Foo: "bar"}, decodeRaw(job.Data, &data{}))
	assert.Equal(t, StatusCompleted, job.Status)
	assert.NotZero(t, job.Created)
	assert.NotZero(t, job.Delayed)
	assert.NotZero(t, job.Started)
	assert.NotZero(t, job.Ended)
	assert.NotZero(t, job.Finished)
	assert.Equal(t, 1, job.Attempts)
	assert.Equal(t, bson.M{"foo": "bar"}, job.Result)
	assert.Equal(t, "", job.Error)
	assert.Equal(t, "", job.Reason)

	p.Close()
}

func TestPoolFailed(t *testing.T) {
	tester.Clean()

	q := &Queue{Store: tester.Store}

	done := make(chan struct{})

	i := 0

	p := NewPool()
	p.Add(&Task{
		Name:  "foo",
		Model: &data{},
		Queue: q,
		Handler: func(m Model) (bson.M, error) {
			if i == 0 {
				i++
				return nil, E("foo", true)
			} else {
				close(done)
				return bson.M{
					"foo": "bar",
				}, nil
			}
		},
		Workers:     2,
		MaxAttempts: 2,
	})
	p.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue("foo", &data{Foo: "bar"}, 0)
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
	assert.Equal(t, "foo", job.Name)
	assert.Equal(t, &data{Foo: "bar"}, decodeRaw(job.Data, &data{}))
	assert.Equal(t, StatusCompleted, job.Status)
	assert.NotZero(t, job.Created)
	assert.NotZero(t, job.Delayed)
	assert.NotZero(t, job.Started)
	assert.NotZero(t, job.Ended)
	assert.NotZero(t, job.Finished)
	assert.Equal(t, 2, job.Attempts)
	assert.Equal(t, bson.M{"foo": "bar"}, job.Result)
	assert.Equal(t, "foo", job.Error)
	assert.Equal(t, "", job.Reason)

	p.Close()
}

func TestPoolCancel(t *testing.T) {
	tester.Clean()

	q := &Queue{Store: tester.Store}

	done := make(chan struct{})

	p := NewPool()
	p.Add(&Task{
		Name:  "cancel",
		Model: &data{},
		Queue: q,
		Handler: func(m Model) (bson.M, error) {
			close(done)
			return nil, E("cancelled", false)
		},
		Workers:     2,
		MaxAttempts: 2,
	})
	p.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue("cancel", &data{Foo: "bar"}, 0)
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
	assert.Equal(t, "cancel", job.Name)
	assert.Equal(t, &data{Foo: "bar"}, decodeRaw(job.Data, &data{}))
	assert.Equal(t, StatusCancelled, job.Status)
	assert.NotZero(t, job.Created)
	assert.NotZero(t, job.Delayed)
	assert.NotZero(t, job.Started)
	assert.NotZero(t, job.Ended)
	assert.NotZero(t, job.Finished)
	assert.Equal(t, 1, job.Attempts)
	assert.Equal(t, bson.M{}, job.Result)
	assert.Equal(t, "", job.Error)
	assert.Equal(t, "cancelled", job.Reason)

	p.Close()
}