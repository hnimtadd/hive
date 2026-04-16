package channel

import (
	"sync"
	"testing"

	agentv1 "github.com/hnimtadd/hive/proto/agent/v1"
)

func TestForTask_CreatesNewChannels(t *testing.T) {
	mgr := NewManager()

	ch := mgr.ForTask("task-1")
	if ch == nil {
		t.Fatal("ForTask returned nil")
	}

	// Verify channels are usable
	select {
	case ch.InputCh <- &agentv1.ExecuteTaskRequest{}:
		// OK
	default:
		t.Fatal("InputCh should accept sends")
	}

	select {
	case ch.OutputCh <- &agentv1.ExecuteTaskResponse{}:
		// OK
	default:
		t.Fatal("OutputCh should accept sends")
	}
}

func TestForTask_ReturnsSameChannels(t *testing.T) {
	mgr := NewManager()

	ch1 := mgr.ForTask("task-1")
	ch2 := mgr.ForTask("task-1")

	if ch1 != ch2 {
		t.Fatal("ForTask should return same channels for same task ID")
	}
}

func TestForTask_DifferentTasksDifferentChannels(t *testing.T) {
	mgr := NewManager()

	ch1 := mgr.ForTask("task-1")
	ch2 := mgr.ForTask("task-2")

	if ch1 == ch2 {
		t.Fatal("Different task IDs should get different channels")
	}
}

func TestCleanup_ClosesChannels(t *testing.T) {
	mgr := NewManager()
	ch := mgr.ForTask("task-1")

	mgr.Cleanup("task-1")

	// OutputCh should be closed
	_, ok := <-ch.OutputCh
	if ok {
		t.Fatal("OutputCh should be closed after cleanup")
	}

	// InputCh should be closed
	_, ok = <-ch.InputCh
	if ok {
		t.Fatal("InputCh should be closed after cleanup")
	}
}

func TestCleanup_Idempotent(t *testing.T) {
	mgr := NewManager()
	mgr.ForTask("task-1")

	// Should not panic on double cleanup
	mgr.Cleanup("task-1")
	mgr.Cleanup("task-1")
}

func TestCleanup_RemovesFromMap(t *testing.T) {
	mgr := NewManager()
	ch1 := mgr.ForTask("task-1")

	mgr.Cleanup("task-1")
	ch2 := mgr.ForTask("task-1")

	if ch1 == ch2 {
		t.Fatal("After cleanup, ForTask should return new channels")
	}
}

func TestConcurrentAccess(t *testing.T) {
	mgr := NewManager()
	var wg sync.WaitGroup

	// Concurrent ForTask calls
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			taskID := "task-1"
			ch := mgr.ForTask(taskID)
			if ch == nil {
				t.Error("ForTask returned nil")
			}
		}(i)
	}

	// Concurrent Cleanup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.Cleanup("task-1")
		}()
	}

	wg.Wait()
}
