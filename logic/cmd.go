package logic

import (
	"fmt"
	"strconv"
	"strings"
	"tgautodown/cmd/tg"
	"tgautodown/internal/logs"
)

const helpText = `可用命令:
/queue - 查看任务进度（任务ID、状态、进度、文件名）
/pause <任务ID> - 暂停任务
/start <任务ID> - 恢复任务
/delete <任务ID> - 删除任务，保留已下载文件
/destroy <任务ID> - 删除任务及已下载文件
/help - 显示本帮助`

var taskStatusText = map[tg.TaskStatus]string{
	tg.TaskQueued:      "排队中",
	tg.TaskDownloading: "下载中",
	tg.TaskPaused:      "已暂停",
	tg.TaskDone:        "已完成",
	tg.TaskFailed:      "失败",
}

func statusText(status tg.TaskStatus) string {
	if s, ok := taskStatusText[status]; ok {
		return s
	}
	return string(status)
}

func handleCommand(ts *tg.TgSuber, msgid int, tgmsg *tg.TgMsg) error {
	fields := strings.Fields(strings.TrimSpace(tgmsg.Text))
	if len(fields) == 0 {
		return nil
	}

	cmd := strings.ToLower(fields[0])
	if i := strings.IndexByte(cmd, '@'); i > 0 { // 群组内 /cmd@botname
		cmd = cmd[:i]
	}
	args := fields[1:]
	mgr := ts.Tasks()

	logs.Debug().Int("msgid", msgid).Str("cmd", cmd).Strs("args", args).Msg("recv command")

	switch cmd {
	case "/queue":
		return ts.ReplyTo(tgmsg, formatQueue(mgr.List()))
	case "/help":
		return ts.ReplyTo(tgmsg, helpText)
	case "/pause", "/start", "/delete", "/destroy":
		id, err := parseTaskID(args)
		if err != nil {
			return ts.ReplyTo(tgmsg, fmt.Sprintf("命令格式错误: %s %s\n- %s", cmd, "<任务ID>", err.Error()))
		}
		return runTaskCommand(ts, tgmsg, mgr, cmd, id)
	default:
		return ts.ReplyTo(tgmsg, fmt.Sprintf("未知命令: %s\n\n%s", cmd, helpText))
	}
}

func runTaskCommand(ts *tg.TgSuber, tgmsg *tg.TgMsg, mgr *tg.TaskMgr, cmd string, id int64) error {
	var err error
	var okMsg string

	switch cmd {
	case "/pause":
		err = mgr.Pause(id)
		okMsg = fmt.Sprintf("任务已暂停: %d", id)
	case "/start":
		err = mgr.Resume(id)
		okMsg = fmt.Sprintf("任务已恢复: %d", id)
	case "/delete":
		err = mgr.Delete(id, false)
		okMsg = fmt.Sprintf("任务已删除（文件保留）: %d", id)
	case "/destroy":
		err = mgr.Delete(id, true)
		okMsg = fmt.Sprintf("任务及文件已删除: %d", id)
	}

	if err != nil {
		return ts.ReplyTo(tgmsg, fmt.Sprintf("操作失败: %s\n- 任务ID: %d\n- 失败原因: %s", cmd, id, err.Error()))
	}
	return ts.ReplyTo(tgmsg, okMsg)
}

func parseTaskID(args []string) (int64, error) {
	if len(args) == 0 {
		return 0, fmt.Errorf("缺少任务ID")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("任务ID必须是正整数")
	}
	return id, nil
}

func formatQueue(list []tg.TaskInfo) string {
	if len(list) == 0 {
		return "当前没有任务"
	}

	var b strings.Builder
	b.WriteString("任务队列:\n")
	for _, t := range list {
		line := fmt.Sprintf("%d | %s | %s | %s\n",
			t.ID, statusText(t.Status), t.Progress(), t.Name)
		if b.Len()+len(line) > 3900 {
			b.WriteString("... (已截断)\n")
			break
		}
		b.WriteString(line)
	}
	return strings.TrimRight(b.String(), "\n")
}
