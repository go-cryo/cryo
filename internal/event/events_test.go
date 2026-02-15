package event

import (
	"testing"

	"github.com/google/uuid"
)

func TestAddEventListenerAndBroadcast(t *testing.T) {
	origListeners := EventListeners
	defer func() { EventListeners = origListeners }()
	EventListeners = nil

	t.Run("matching event is received", func(t *testing.T) {
		EventListeners = nil
		received := false
		AddEventListener(&EventListener{
			Object: EventObjectBackupJob,
			Action: EventActionCreate,
			Handle: func(e *Event) { received = true },
		})
		BroadcastEvent(&Event{
			Object: EventObjectBackupJob,
			Action: EventActionCreate,
			Id:     uuid.New(),
		})
		if !received {
			t.Error("listener should have received matching event")
		}
	})

	t.Run("wildcard object listener receives all objects", func(t *testing.T) {
		EventListeners = nil
		count := 0
		AddEventListener(&EventListener{
			Object: EventObjectAny,
			Action: EventActionCreate,
			Handle: func(e *Event) { count++ },
		})
		BroadcastEvent(&Event{Object: EventObjectBackupJob, Action: EventActionCreate, Id: uuid.New()})
		BroadcastEvent(&Event{Object: EventObjectBackupRun, Action: EventActionCreate, Id: uuid.New()})
		if count != 2 {
			t.Errorf("wildcard object listener received %d events, want 2", count)
		}
	})

	t.Run("wildcard action listener receives all actions", func(t *testing.T) {
		EventListeners = nil
		count := 0
		AddEventListener(&EventListener{
			Object: EventObjectBackupJob,
			Action: EventActionAny,
			Handle: func(e *Event) { count++ },
		})
		BroadcastEvent(&Event{Object: EventObjectBackupJob, Action: EventActionCreate, Id: uuid.New()})
		BroadcastEvent(&Event{Object: EventObjectBackupJob, Action: EventActionUpdate, Id: uuid.New()})
		BroadcastEvent(&Event{Object: EventObjectBackupJob, Action: EventActionDelete, Id: uuid.New()})
		if count != 3 {
			t.Errorf("wildcard action listener received %d events, want 3", count)
		}
	})

	t.Run("non-matching event is ignored", func(t *testing.T) {
		EventListeners = nil
		received := false
		AddEventListener(&EventListener{
			Object: EventObjectBackupJob,
			Action: EventActionCreate,
			Handle: func(e *Event) { received = true },
		})
		BroadcastEvent(&Event{
			Object: EventObjectBackupRun,
			Action: EventActionDelete,
			Id:     uuid.New(),
		})
		if received {
			t.Error("listener should not have received non-matching event")
		}
	})

	t.Run("multiple listeners all receive matching events", func(t *testing.T) {
		EventListeners = nil
		count1, count2 := 0, 0
		AddEventListener(&EventListener{
			Object: EventObjectBackupJob,
			Action: EventActionCreate,
			Handle: func(e *Event) { count1++ },
		})
		AddEventListener(&EventListener{
			Object: EventObjectBackupJob,
			Action: EventActionCreate,
			Handle: func(e *Event) { count2++ },
		})
		BroadcastEvent(&Event{Object: EventObjectBackupJob, Action: EventActionCreate, Id: uuid.New()})
		if count1 != 1 {
			t.Errorf("listener 1 received %d events, want 1", count1)
		}
		if count2 != 1 {
			t.Errorf("listener 2 received %d events, want 1", count2)
		}
	})

	t.Run("event passes correct data to handler", func(t *testing.T) {
		EventListeners = nil
		id := uuid.New()
		var receivedEvent *Event
		AddEventListener(&EventListener{
			Object: EventObjectBackupJob,
			Action: EventActionUpdate,
			Handle: func(e *Event) { receivedEvent = e },
		})
		BroadcastEvent(&Event{Object: EventObjectBackupJob, Action: EventActionUpdate, Id: id})
		if receivedEvent == nil {
			t.Fatal("listener did not receive event")
		}
		if receivedEvent.Id != id {
			t.Errorf("event Id = %v, want %v", receivedEvent.Id, id)
		}
		if receivedEvent.Object != EventObjectBackupJob {
			t.Errorf("event Object = %v, want %v", receivedEvent.Object, EventObjectBackupJob)
		}
		if receivedEvent.Action != EventActionUpdate {
			t.Errorf("event Action = %v, want %v", receivedEvent.Action, EventActionUpdate)
		}
	})
}
