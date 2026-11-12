package tg

import (
	"path/filepath"
	"testing"

	"github.com/gotd/td/tg"
)

func newTestMgr(t *testing.T, store string) *TaskMgr {
	t.Helper()
	return newTaskMgr(&TgSuber{}, store, 2)
}

func newTestMsg(name string, size int64) *TgMsg {
	return &TgMsg{
		From:     &SubChannelInfo{Name: "ch", Title: "channel", ChannelID: 1, chType: ChChannel},
		FileName: name,
		FileSize: size,
		mcls:     TgVideo,
		msg:      &tg.Message{ID: 10},
	}
}

func TestTaskLifecycle(t *testing.T) {
	store := filepath.Join(t.TempDir(), "tasks.json")
	m := newTestMgr(t, store)

	id1 := m.add(newTestMsg("a.mp4", 100), "/tmp/a.mp4", nil)
	id2 := m.add(newTestMsg("b.mp4", 200), "/tmp/b.mp4", nil)
	id3 := m.add(newTestMsg("c.mp4", 300), "/tmp/c.mp4", nil)
	if id1 != 1 || id2 != 2 || id3 != 3 {
		t.Fatalf("task ids = %d,%d,%d, want 1,2,3", id1, id2, id3)
	}
	if len(m.List()) != 3 {
		t.Fatalf("list len = %d, want 3", len(m.List()))
	}

	if err := m.Pause(id2); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if info, _ := m.Get(id2); info.Status != TaskPaused {
		t.Fatalf("status = %s, want paused", info.Status)
	}
	if len(m.queue) != 2 {
		t.Fatalf("queue len = %d, want 2", len(m.queue))
	}

	if err := m.Resume(id2); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if info, _ := m.Get(id2); info.Status != TaskQueued {
		t.Fatalf("status = %s, want queued", info.Status)
	}

	if err := m.Delete(id1, false); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := m.Get(id1); ok {
		t.Fatal("task 1 still exists")
	}
	if err := m.Delete(id1, false); err != ErrTaskNotFound {
		t.Fatalf("delete missing task err = %v, want ErrTaskNotFound", err)
	}
}

func TestTaskPriorityOrder(t *testing.T) {
	m := newTestMgr(t, "")
	low := m.add(newTestMsg("low.mp4", 1), "/tmp/low.mp4", nil)
	high := m.add(newTestMsg("high.mp4", 1), "/tmp/high.mp4", nil)

	m.mu.Lock()
	m.tasks[low].Priority = TaskPrioLow
	m.mu.Unlock()
	// 重新整理队列以应用新优先级
	if err := m.Pause(low); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if err := m.Resume(low); err != nil {
		t.Fatalf("resume: %v", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.queue) != 2 {
		t.Fatalf("queue len = %d, want 2", len(m.queue))
	}
	if m.queue[0].ID != high || m.queue[1].ID != low {
		t.Fatalf("queue order = %d,%d, want %d,%d", m.queue[0].ID, m.queue[1].ID, high, low)
	}
}

func TestTaskPersistAndRestore(t *testing.T) {
	store := filepath.Join(t.TempDir(), "tasks.json")
	m := newTestMgr(t, store)

	id1 := m.add(newTestMsg("a.mp4", 100), "/tmp/a.mp4", nil)
	id2 := m.add(newTestMsg("b.mp4", 200), "/tmp/b.mp4", nil)
	id3 := m.add(newTestMsg("c.mp4", 300), "/tmp/c.mp4", nil)

	if err := m.Pause(id3); err != nil {
		t.Fatalf("pause: %v", err)
	}
	m.mu.Lock()
	m.tasks[id2].Status = TaskDone
	m.mu.Unlock()
	m.persist()

	restored := newTestMgr(t, store)
	restored.restore()

	if got := len(restored.tasks); got != 2 {
		t.Fatalf("restored tasks = %d, want 2 (done task excluded)", got)
	}
	if _, ok := restored.tasks[id2]; ok {
		t.Fatal("done task should not be restored")
	}
	if info, ok := restored.Get(id1); !ok || info.Status != TaskQueued {
		t.Fatalf("task %d restored as %+v, want queued", id1, info)
	}
	if info, ok := restored.Get(id3); !ok || info.Status != TaskPaused {
		t.Fatalf("task %d restored as %+v, want paused", id3, info)
	}
	if info, _ := restored.Get(id3); info.Name != "c.mp4" || info.FileSize != 300 {
		t.Fatalf("task %d record not restored: %+v", id3, info)
	}

	newID := restored.add(newTestMsg("d.mp4", 400), "/tmp/d.mp4", nil)
	if newID != 4 {
		t.Fatalf("next id = %d, want 4", newID)
	}
}
