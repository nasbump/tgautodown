package tg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"tgautodown/internal/logs"
	"time"

	"github.com/gotd/td/tg"
)

type TaskStatus string

const (
	TaskQueued      TaskStatus = "queued"
	TaskDownloading TaskStatus = "downloading"
	TaskPaused      TaskStatus = "paused"
	TaskDone        TaskStatus = "done"
	TaskFailed      TaskStatus = "failed"
)

const (
	TaskPrioNormal = 0
	TaskPrioLow    = 1
)

var (
	ErrTaskNotFound = errors.New("task not found")
	ErrTaskState    = errors.New("task state not allowed")
)

// taskRecord 是任务的可落盘描述，不含运行时上下文。
type taskRecord struct {
	ID         int64      `json:"id"`
	Status     TaskStatus `json:"status"`
	Priority   int        `json:"priority"`
	Name       string     `json:"name"`
	SavePath   string     `json:"savePath"`
	FileSize   int64      `json:"fileSize"`
	RetryCnt   int        `json:"retryCnt"`
	CreatedAt  time.Time  `json:"createdAt"`
	MsgID      int        `json:"msgId"`
	ChannelID  int64      `json:"channelId"`
	AccessHash int64      `json:"accessHash"`
	ChName     string     `json:"chName"`
	ChTitle    string     `json:"chTitle"`
	ChType     int        `json:"chType"`
	Mcls       TgMsgClass `json:"mcls"`
	Ptype      string     `json:"ptype"`
}

type taskStore struct {
	NextID int64        `json:"nextId"`
	Tasks  []taskRecord `json:"tasks"`
}

// TaskInfo 是任务对外快照，供命令层展示。
type TaskInfo struct {
	ID         int64
	Status     TaskStatus
	Priority   int
	Name       string
	SavePath   string
	FileSize   int64
	Downloaded int64
	RetryCnt   int
	CreatedAt  time.Time
}

func (t TaskInfo) Progress() string {
	if t.FileSize <= 0 {
		return "-"
	}
	done := t.Downloaded
	if done > t.FileSize {
		done = t.FileSize
	}
	if done < 0 {
		done = 0
	}
	return fmt.Sprintf("%.1f%%", float64(done)*100/float64(t.FileSize))
}

type Task struct {
	ID         int64
	Status     TaskStatus
	Priority   int
	Name       string
	SavePath   string
	FileSize   int64
	Downloaded atomic.Int64
	RetryCnt   int
	CreatedAt  time.Time

	record taskRecord
	tgmsg  *TgMsg
	ctx    context.Context
	cancel context.CancelFunc
	done   TgOnSaveDoneHnd
	finish chan struct{}
	busy   bool // 有 worker 正在执行该任务
}

type TaskMgr struct {
	mu        sync.Mutex
	cond      *sync.Cond
	persistMu sync.Mutex
	tasks     map[int64]*Task
	queue     []*Task
	nextID    int64
	maxLoad   int
	running   int
	baseCtx   context.Context
	store     string
	owner     *TgSuber
	startOne  sync.Once
}

func newTaskMgr(owner *TgSuber, store string, maxLoad int) *TaskMgr {
	if maxLoad < 1 {
		maxLoad = 1
	}
	m := &TaskMgr{
		tasks:   map[int64]*Task{},
		maxLoad: maxLoad,
		store:   store,
		owner:   owner,
		nextID:  1,
	}
	m.cond = sync.NewCond(&m.mu)
	return m
}

func (m *TaskMgr) start() {
	m.startOne.Do(func() {
		go m.scheduleLoop()
	})
}

// SetBaseCtx 由连接层在重连时调用；nil 表示连接不可用，暂停派发。
func (m *TaskMgr) SetBaseCtx(ctx context.Context) {
	m.mu.Lock()
	m.baseCtx = ctx
	m.mu.Unlock()
	m.cond.Broadcast()
}

func (m *TaskMgr) scheduleLoop() {
	for {
		m.mu.Lock()
		for {
			idx := -1
			if m.baseCtx != nil && m.baseCtx.Err() == nil && m.running < m.maxLoad {
				for i, t := range m.queue {
					if t.Status == TaskQueued && !t.busy {
						idx = i
						break
					}
				}
			}
			if idx < 0 {
				m.cond.Wait()
				continue
			}

			task := m.queue[idx]
			m.queue = append(m.queue[:idx], m.queue[idx+1:]...)
			task.Status = TaskDownloading
			task.ctx, task.cancel = context.WithCancel(m.baseCtx)
			task.finish = make(chan struct{})
			task.busy = true
			m.running++
			m.mu.Unlock()

			go m.runTask(task)
			break
		}
	}
}

func (m *TaskMgr) runTask(t *Task) {
	m.mu.Lock()
	runCtx, runFinish := t.ctx, t.finish
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.running--
		t.busy = false
		if runFinish != nil && t.finish == runFinish {
			close(t.finish)
			t.finish = nil
		}
		m.mu.Unlock()
		m.cond.Broadcast()
	}()

	err := m.owner.downloadTask(t)
	if err == nil {
		m.mu.Lock()
		stale := t.ctx != runCtx
		m.mu.Unlock()
		if !stale {
			m.finishTask(t, nil)
		}
		return
	}

	m.mu.Lock()
	if t.ctx != runCtx {
		// 该次运行已被暂停/恢复打断，状态由新的一轮接管
		m.mu.Unlock()
		return
	}
	ctx := t.ctx
	if ctx != nil && ctx.Err() != nil {
		// 暂停/删除/连接中断：断点已保留，连接恢复后重新入队
		if t.Status == TaskDownloading {
			t.Status = TaskQueued
			m.enqueueLocked(t)
			m.mu.Unlock()
			m.persist()
			return
		}
		m.mu.Unlock()
		return
	}
	if t.Status != TaskDownloading {
		// 已被暂停或删除，不再重新入队
		m.mu.Unlock()
		return
	}

	t.RetryCnt++
	if m.owner.MaxSaveRetryCnt > 0 && t.RetryCnt > m.owner.MaxSaveRetryCnt {
		m.mu.Unlock()
		m.finishTask(t, err)
		return
	}

	// 重试 1 次后降低优先级重新排队
	t.Status = TaskQueued
	t.Priority = TaskPrioLow
	m.enqueueLocked(t)
	m.mu.Unlock()
	m.persist()

	logs.Warn(err).Int64("task.id", t.ID).Int("retry", t.RetryCnt).Msg("download task requeue with low priority")
}

func (m *TaskMgr) finishTask(t *Task, err error) {
	m.mu.Lock()
	if m.tasks[t.ID] != t {
		m.mu.Unlock()
		return // 任务已被删除
	}
	if err != nil {
		t.Status = TaskFailed
	} else {
		t.Status = TaskDone
		t.Downloaded.Store(t.FileSize)
	}
	t.cancel = nil
	done := t.done
	m.mu.Unlock()

	m.persist()
	if done != nil {
		t.tgmsg.TaskID = t.ID
		done(m.owner, t.SavePath, t.tgmsg.msg.ID, t.tgmsg, err)
	}
}

func (m *TaskMgr) enqueueLocked(t *Task) {
	t.tgmsg.TaskID = t.ID
	m.queue = append(m.queue, t)
	sort.SliceStable(m.queue, func(i, j int) bool {
		if m.queue[i].Priority != m.queue[j].Priority {
			return m.queue[i].Priority < m.queue[j].Priority
		}
		return m.queue[i].ID < m.queue[j].ID
	})
}

func (m *TaskMgr) add(tgmsg *TgMsg, savePath string, done TgOnSaveDoneHnd) int64 {
	m.mu.Lock()
	t := &Task{
		ID:        m.nextID,
		Status:    TaskQueued,
		Priority:  TaskPrioNormal,
		Name:      tgmsg.FileName,
		SavePath:  savePath,
		FileSize:  tgmsg.FileSize,
		CreatedAt: time.Now(),
		tgmsg:     tgmsg,
		done:      done,
		finish:    make(chan struct{}),
	}
	m.nextID++
	m.refreshRecordLocked(t)
	m.tasks[t.ID] = t
	m.enqueueLocked(t)
	m.mu.Unlock()

	m.persist()
	m.cond.Broadcast()
	return t.ID
}

func (m *TaskMgr) List() []TaskInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	list := make([]TaskInfo, 0, len(m.tasks))
	for _, t := range m.tasks {
		list = append(list, TaskInfo{
			ID:         t.ID,
			Status:     t.Status,
			Priority:   t.Priority,
			Name:       t.Name,
			SavePath:   t.SavePath,
			FileSize:   t.FileSize,
			Downloaded: t.Downloaded.Load(),
			RetryCnt:   t.RetryCnt,
			CreatedAt:  t.CreatedAt,
		})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

func (m *TaskMgr) Get(id int64) (TaskInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return TaskInfo{}, false
	}
	return TaskInfo{
		ID:         t.ID,
		Status:     t.Status,
		Priority:   t.Priority,
		Name:       t.Name,
		SavePath:   t.SavePath,
		FileSize:   t.FileSize,
		Downloaded: t.Downloaded.Load(),
		RetryCnt:   t.RetryCnt,
		CreatedAt:  t.CreatedAt,
	}, true
}

func (m *TaskMgr) Pause(id int64) error {
	m.mu.Lock()
	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return ErrTaskNotFound
	}
	switch t.Status {
	case TaskQueued:
		t.Status = TaskPaused
		m.removeLocked(t)
		m.mu.Unlock()
		m.persist()
		return nil
	case TaskDownloading:
		t.Status = TaskPaused
		cancel := t.cancel
		m.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		m.persist()
		return nil
	default:
		m.mu.Unlock()
		return ErrTaskState
	}
}

func (m *TaskMgr) Resume(id int64) error {
	m.mu.Lock()
	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return ErrTaskNotFound
	}
	if t.Status != TaskPaused {
		m.mu.Unlock()
		return ErrTaskState
	}
	t.Status = TaskQueued
	m.enqueueLocked(t)
	m.mu.Unlock()

	m.persist()
	m.cond.Broadcast()
	return nil
}

func (m *TaskMgr) Delete(id int64, removeFiles bool) error {
	m.mu.Lock()
	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return ErrTaskNotFound
	}
	wasRunning := t.busy
	delete(m.tasks, id)
	m.removeLocked(t)
	t.Status = TaskPaused
	cancel, finish := t.cancel, t.finish
	m.mu.Unlock()

	if wasRunning {
		if cancel != nil {
			cancel()
		}
		if finish != nil {
			<-finish
		}
	}

	if removeFiles {
		removeTaskFiles(t.SavePath)
	}
	m.persist()
	return nil
}

func removeTaskFiles(savePath string) {
	if savePath == "" {
		return
	}
	for _, p := range []string{savePath, savePath + ".dl"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			logs.Warn(err).Str("file", p).Msg("remove task file fail")
		}
	}
}

func (m *TaskMgr) removeLocked(t *Task) {
	for i, q := range m.queue {
		if q == t {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			break
		}
	}
}

func (m *TaskMgr) refreshRecordLocked(t *Task) {
	rec := taskRecord{
		ID:        t.ID,
		Status:    t.Status,
		Priority:  t.Priority,
		Name:      t.Name,
		SavePath:  t.SavePath,
		FileSize:  t.FileSize,
		RetryCnt:  t.RetryCnt,
		CreatedAt: t.CreatedAt,
		MsgID:     t.tgmsg.msg.ID,
		Mcls:      t.tgmsg.mcls,
		Ptype:     t.tgmsg.ptype,
	}
	if from := t.tgmsg.From; from != nil {
		rec.ChannelID = from.ChannelID
		rec.AccessHash = from.AccessHash
		rec.ChName = from.Name
		rec.ChTitle = from.Title
		rec.ChType = from.chType
	}
	t.record = rec
}

func (m *TaskMgr) persist() {
	if m.store == "" {
		return
	}

	// 串行化写入，避免较早的快照覆盖较新的状态
	m.persistMu.Lock()
	defer m.persistMu.Unlock()

	m.mu.Lock()
	store := taskStore{NextID: m.nextID}
	for _, t := range m.tasks {
		if t.Status == TaskDone || t.Status == TaskFailed {
			continue
		}
		m.refreshRecordLocked(t)
		store.Tasks = append(store.Tasks, t.record)
	}
	m.mu.Unlock()

	if err := writeJSONAtomic(m.store, &store); err != nil {
		logs.Warn(err).Str("file", m.store).Msg("save tasks fail")
	}
}

func (m *TaskMgr) restore() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.store == "" {
		return
	}
	data, err := os.ReadFile(m.store)
	if err != nil {
		if !os.IsNotExist(err) {
			logs.Warn(err).Str("file", m.store).Msg("read tasks fail")
		}
		return
	}

	var store taskStore
	if err := json.Unmarshal(data, &store); err != nil {
		logs.Warn(err).Str("file", m.store).Msg("decode tasks fail")
		return
	}

	maxID := int64(0)
	for _, rec := range store.Tasks {
		if rec.ID > maxID {
			maxID = rec.ID
		}
		if rec.Status == TaskDone || rec.Status == TaskFailed {
			continue
		}
		status := TaskQueued
		if rec.Status == TaskPaused {
			status = TaskPaused
		}
		t := &Task{
			ID:        rec.ID,
			Status:    status,
			Priority:  rec.Priority,
			Name:      rec.Name,
			SavePath:  rec.SavePath,
			FileSize:  rec.FileSize,
			RetryCnt:  rec.RetryCnt,
			CreatedAt: rec.CreatedAt,
			record:    rec,
			tgmsg: &TgMsg{
				From: &SubChannelInfo{
					Name:       rec.ChName,
					Title:      rec.ChTitle,
					ChannelID:  rec.ChannelID,
					AccessHash: rec.AccessHash,
					chType:     rec.ChType,
				},
				FileName: rec.Name,
				FileSize: rec.FileSize,
				mcls:     rec.Mcls,
				ptype:    rec.Ptype,
				msg:      &tg.Message{ID: rec.MsgID},
			},
			finish: make(chan struct{}),
		}
		t.Downloaded.Store(fileSize(t.SavePath + ".dl"))
		m.tasks[t.ID] = t
		if status == TaskQueued {
			m.enqueueLocked(t)
		}
	}

	if store.NextID > maxID {
		m.nextID = store.NextID
	} else {
		m.nextID = maxID + 1
	}

	if len(m.tasks) > 0 {
		logs.Info().Int("tasks", len(m.tasks)).Int64("nextid", m.nextID).Msg("restore tasks")
	}
	m.cond.Broadcast()
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func writeJSONAtomic(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".tasks-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, path)
}
