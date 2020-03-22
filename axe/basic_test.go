package axe

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestQueueing(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job.ID())

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)

		model := list[0]
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "simple",
			Data: coal.Map{
				"data": "Hello!",
			},
			Status:    Enqueued,
			Created:   model.Created,
			Available: model.Available,
			Events: []Event{
				{
					Timestamp: model.Created,
					Status:    Enqueued,
				},
			},
		}, model)

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		model = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "simple",
			Data: coal.Map{
				"data": "Hello!",
			},
			Status:    Dequeued,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Attempts:  1,
			Events: []Event{
				{
					Timestamp: model.Created,
					Status:    Enqueued,
				},
				{
					Timestamp: *model.Started,
					Status:    Dequeued,
				},
			},
		}, model)

		job.Data = "Hello!!!"
		err = Complete(nil, tester.Store, &job)
		assert.NoError(t, err)

		model = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.NotNil(t, model.Ended)
		assert.NotNil(t, model.Finished)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "simple",
			Data: coal.Map{
				"data": "Hello!!!",
			},
			Status:    Completed,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Ended:     model.Ended,
			Finished:  model.Finished,
			Attempts:  1,
			Events: []Event{
				{
					Timestamp: model.Created,
					Status:    Enqueued,
				},
				{
					Timestamp: *model.Started,
					Status:    Dequeued,
				},
				{
					Timestamp: *model.Finished,
					Status:    Completed,
				},
			},
		}, model)
	})
}

func TestQueueingDelayed(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 100*time.Millisecond, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job.ID())

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)

		time.Sleep(200 * time.Millisecond)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "simple",
			Data: coal.Map{
				"data": "Hello!",
			},
			Status:    Dequeued,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Attempts:  1,
			Events: []Event{
				{
					Timestamp: model.Created,
					Status:    Enqueued,
				},
				{
					Timestamp: *model.Started,
					Status:    Dequeued,
				},
			},
		}, model)
	})
}

func TestDequeueTimeout(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job.ID())

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)

		time.Sleep(200 * time.Millisecond)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 2, attempt)

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.NotZero(t, model.Events[1].Timestamp)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "simple",
			Data: coal.Map{
				"data": "Hello!",
			},
			Status:    Dequeued,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Attempts:  2,
			Events: []Event{
				{
					Timestamp: model.Created,
					Status:    Enqueued,
				},
				{
					Timestamp: model.Events[1].Timestamp,
					Status:    Dequeued,
				},
				{
					Timestamp: *model.Started,
					Status:    Dequeued,
				},
			},
		}, model)
	})
}

func TestFail(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job.ID())

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		err = Fail(nil, tester.Store, &job, "some error", 0)
		assert.NoError(t, err)

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.NotNil(t, model.Ended)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "simple",
			Data: coal.Map{
				"data": "Hello!",
			},
			Status:    Failed,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Ended:     model.Ended,
			Attempts:  1,
			Events: []Event{
				{
					Timestamp: model.Created,
					Status:    Enqueued,
				},
				{
					Timestamp: *model.Started,
					Status:    Dequeued,
				},
				{
					Timestamp: *model.Ended,
					Status:    Failed,
					Reason:    "some error",
				},
			},
		}, model)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 2, attempt)

		model = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.NotZero(t, model.Events[1].Timestamp)
		assert.NotZero(t, model.Events[2].Timestamp)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "simple",
			Data: coal.Map{
				"data": "Hello!",
			},
			Status:    Dequeued,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Attempts:  2,
			Events: []Event{
				{
					Timestamp: model.Created,
					Status:    Enqueued,
				},
				{
					Timestamp: model.Events[1].Timestamp,
					Status:    Dequeued,
				},
				{
					Timestamp: model.Events[2].Timestamp,
					Status:    Failed,
					Reason:    "some error",
				},
				{
					Timestamp: *model.Started,
					Status:    Dequeued,
				},
			},
		}, model)
	})
}

func TestFailDelayed(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job.ID())

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		err = Fail(nil, tester.Store, &job, "some error", 100*time.Millisecond)
		assert.NoError(t, err)

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.NotNil(t, model.Ended)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "simple",
			Data: coal.Map{
				"data": "Hello!",
			},
			Status:    Failed,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Ended:     model.Ended,
			Attempts:  1,
			Events: []Event{
				{
					Timestamp: model.Created,
					Status:    Enqueued,
				},
				{
					Timestamp: *model.Started,
					Status:    Dequeued,
				},
				{
					Timestamp: *model.Ended,
					Status:    Failed,
					Reason:    "some error",
				},
			},
		}, model)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)

		time.Sleep(200 * time.Millisecond)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 2, attempt)

		model = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.NotZero(t, model.Events[1].Timestamp)
		assert.NotZero(t, model.Events[2].Timestamp)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "simple",
			Data: coal.Map{
				"data": "Hello!",
			},
			Status:    Dequeued,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Attempts:  2,
			Events: []Event{
				{
					Timestamp: model.Created,
					Status:    Enqueued,
				},
				{
					Timestamp: model.Events[1].Timestamp,
					Status:    Dequeued,
				},
				{
					Timestamp: model.Events[2].Timestamp,
					Status:    Failed,
					Reason:    "some error",
				},
				{
					Timestamp: *model.Started,
					Status:    Dequeued,
				},
			},
		}, model)
	})
}

func TestCancel(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job.ID())

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		err = Cancel(nil, tester.Store, &job, "some reason")
		assert.NoError(t, err)

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.NotNil(t, model.Ended)
		assert.NotNil(t, model.Finished)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "simple",
			Data: coal.Map{
				"data": "Hello!",
			},
			Status:    Cancelled,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Ended:     model.Ended,
			Finished:  model.Finished,
			Attempts:  1,
			Events: []Event{
				{
					Timestamp: model.Created,
					Status:    Enqueued,
				},
				{
					Timestamp: *model.Started,
					Status:    Dequeued,
				},
				{
					Timestamp: *model.Ended,
					Status:    Cancelled,
					Reason:    "some reason",
				},
			},
		}, model)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)
	})
}

func TestEnqueueLabeled(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job1 := simpleJob{
			Base: B("test"),
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job1, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job1.ID())

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, Enqueued, list[0].Status)

		job2 := simpleJob{
			Base: B("test"),
			Data: "Hello!",
		}

		enqueued, err = Enqueue(nil, tester.Store, &job2, 0, 0)
		assert.NoError(t, err)
		assert.False(t, enqueued)
		assert.NotZero(t, job2.ID())

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, Enqueued, list[0].Status)

		_, _, err = Dequeue(nil, tester.Store, &job1, time.Second)
		assert.NoError(t, err)

		err = Complete(nil, tester.Store, &job1)
		assert.NoError(t, err)

		enqueued, err = Enqueue(nil, tester.Store, &job2, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job2.ID())

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 2)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, Completed, list[0].Status)
		assert.Equal(t, "simple", list[1].Name)
		assert.Equal(t, "test", list[1].Label)
		assert.Equal(t, Enqueued, list[1].Status)
	})
}

func TestEnqueueIsolation(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job1 := simpleJob{
			Base: B("test"),
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job1, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job1.ID())

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, Enqueued, list[0].Status)

		job2 := simpleJob{
			Base: B("test"),
			Data: "Hello!",
		}

		enqueued, err = Enqueue(nil, tester.Store, &job2, 0, 0)
		assert.NoError(t, err)
		assert.False(t, enqueued)
		assert.NotZero(t, job2.ID())

		enqueued, err = Enqueue(nil, tester.Store, &job2, 0, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.False(t, enqueued)
		assert.NotZero(t, job2.ID())

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, Enqueued, list[0].Status)

		_, _, err = Dequeue(nil, tester.Store, &job1, time.Second)
		assert.NoError(t, err)

		err = Complete(nil, tester.Store, &job1)
		assert.NoError(t, err)

		enqueued, err = Enqueue(nil, tester.Store, &job2, 0, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.False(t, enqueued)
		assert.NotZero(t, job2.ID())

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, Completed, list[0].Status)

		time.Sleep(200 * time.Millisecond)

		enqueued, err = Enqueue(nil, tester.Store, &job2, 0, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job2.ID())

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 2)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, Completed, list[0].Status)
		assert.Equal(t, "simple", list[1].Name)
		assert.Equal(t, "test", list[1].Label)
		assert.Equal(t, Enqueued, list[1].Status)
	})
}
