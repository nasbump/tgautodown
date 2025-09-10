package logic

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"tgautodown/cmd/tg"
	"tgautodown/internal/logs"
)

var Tgs *tg.TgSuber

var namesMap = map[tg.TgMsgClass][]string{
	tg.TgVideo:    {"videos", "视频"},
	tg.TgAudio:    {"music", "音乐"},
	tg.TgDocument: {"documents", "文档"},
	tg.TgPhoto:    {"photos", "照片"},
	tg.TgNote:     {"note", "笔记"},
	"bt":          {"bt", "BT"},
}

func TgSuberStart() {
	Tgs = tg.NewTG(TGCfg.AppID, TGCfg.AppHash, TGCfg.Phone).
		WithSocks5Proxy(TGCfg.socks5).
		WithSession(TGCfg.sessionPath, TGCfg.f2apwd, waitLoginCode)
		// WithHistoryMsgCnt(2).

	Tgs.WithMsgHandle(tg.TgAudio, func(msgid int, tgmsg *tg.TgMsg) error {
		return doDownload(Tgs, tg.TgAudio, msgid, tgmsg)
	})
	Tgs.WithMsgHandle(tg.TgDocument, func(msgid int, tgmsg *tg.TgMsg) error {
		return doDownload(Tgs, tg.TgDocument, msgid, tgmsg)
	})
	Tgs.WithMsgHandle(tg.TgVideo, func(msgid int, tgmsg *tg.TgMsg) error {
		return doDownload(Tgs, tg.TgVideo, msgid, tgmsg)
	})
	Tgs.WithMsgHandle(tg.TgPhoto, func(msgid int, tgmsg *tg.TgMsg) error {
		return doDownload(Tgs, tg.TgPhoto, msgid, tgmsg)
	})
	Tgs.WithMsgHandle(tg.TgNote, func(msgid int, tgmsg *tg.TgMsg) error {
		if strings.HasPrefix(strings.ToLower(tgmsg.Text), "magnet:?") {
			return downloadMagnet(Tgs, "bt", msgid, tgmsg)
		} else {
			return writeNote(Tgs, tg.TgNote, msgid, tgmsg)
		}
	})

	Tgs.Run(TGCfg.channelNames)
}

var codech chan string

func init() {
	codech = make(chan string)
}

func waitLoginCode() string {
	logs.Info().Msg("waiting for login.code...")
	return <-codech
}
func InputLoginCode(code string) {
	logs.Info().Str("login.code", code).Msg("input")
	codech <- code
}

func doDownload(ts *tg.TgSuber, mtype tg.TgMsgClass, msgid int, tgmsg *tg.TgMsg) error {
	subDir := namesMap[mtype][0]
	mtDesc := namesMap[mtype][1]

	replyMsg := fmt.Sprintf("正在下载%s: %s\n- 文件大小: %s\n- 消息ID: %d",
		mtDesc, tgmsg.FileName, sizeInt2Readable(tgmsg.FileSize), msgid)
	logs.Debug().Msg(replyMsg)
	ts.ReplyTo(tgmsg, replyMsg)
	savePath := getSavePath(subDir, tgmsg.FileName)
	if err := ts.SaveFile(tgmsg, savePath); err != nil {
		replyMsg = fmt.Sprintf("下载失败: %s\n- 消息ID: %d\n- 失败原因: %s",
			tgmsg.FileName, msgid, err.Error())
	} else {
		replyMsg = fmt.Sprintf("下载成功: %s\n- 消息ID: %d\n- 保存路径: %s",
			tgmsg.FileName, msgid, savePath)
	}
	logs.Debug().Str("from", tgmsg.From.Title).Msg(replyMsg)
	return ts.ReplyTo(tgmsg, replyMsg)
}

func getSavePath(mtype, filename string) string {
	savePath := filepath.Join(TGCfg.SaveDir, mtype)
	createDir(savePath)
	savePath = filepath.Join(savePath, filename)
	return uniquePath(savePath)
}

func downloadMagnet(ts *tg.TgSuber, mtype tg.TgMsgClass, msgid int, tgmsg *tg.TgMsg) error {
	subDir := namesMap[mtype][0]
	mtDesc := namesMap[mtype][1]
	url := tgmsg.Text
	logs.Debug().Int("msgid", msgid).Str("url", url).Str("from", tgmsg.From.Title).Msg("recv magnet")

	replyMsg := fmt.Sprintf("正在下载%s:\n- 消息ID: %d", mtDesc, msgid)
	ts.ReplyTo(tgmsg, replyMsg)

	savePath := filepath.Join(TGCfg.SaveDir, subDir)
	createDir(savePath)

	err := exec.Command(TGCfg.Gopeed, "-C", "32", "-D", savePath, url).Run()
	if err != nil {
		replyMsg = fmt.Sprintf("%s下载失败:\n- 消息ID: %d\n- 失败原因: %s",
			mtDesc, msgid, err.Error())
	} else {
		replyMsg = fmt.Sprintf("%s下载成功:\n- 消息ID: %d\n- 保存路径: %s",
			mtDesc, msgid, savePath)
	}
	logs.Debug().Msg(replyMsg)
	return ts.ReplyTo(tgmsg, replyMsg)
}

func writeNote(ts *tg.TgSuber, mtype tg.TgMsgClass, msgid int, tgmsg *tg.TgMsg) error {
	subDir := namesMap[mtype][0]
	mtDesc := namesMap[mtype][1]
	note := tgmsg.Text
	logs.Debug().Int("msgid", msgid).Str("note", note).Str("from", tgmsg.From.Title).Msg("recv note")

	savePath := filepath.Join(TGCfg.SaveDir, subDir)
	createDir(savePath)
	savePath = filepath.Join(savePath, strconv.FormatInt(int64(msgid), 10)+".md")

	err := os.WriteFile(savePath, []byte(note), 0666)
	replyMsg := ""
	if err != nil {
		replyMsg = fmt.Sprintf("%s添加失败:\n- 消息ID: %d\n- 失败原因: %s",
			mtDesc, msgid, err.Error())
	} else {
		replyMsg = fmt.Sprintf("%s添加成功:\n- 消息ID: %d\n- 保存路径: %s",
			mtDesc, msgid, savePath)
	}
	logs.Debug().Msg(replyMsg)
	return ts.ReplyTo(tgmsg, replyMsg)
}

func createDir(dir string) error {
	if fs, err := os.Stat(dir); err == nil {
		if fs.IsDir() {
			return nil
		}
		return fmt.Errorf("same file had been existed")
	}

	return os.MkdirAll(dir, 0777)
}

func sizeInt2Readable(size int64) string {
	if (size >> 30) > 0 {
		return fmt.Sprintf("%.2fGB", float64(size)/1073741824.0)
	}
	if (size >> 20) > 0 {
		return fmt.Sprintf("%.2fMB", float64(size)/1048576.0)
	}
	if (size >> 10) > 0 {
		return fmt.Sprintf("%.2fKB", float64(size)/1024.0)
	}
	return fmt.Sprintf("%d Bytes", size)
}

// 防止重名
func uniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	name := path[:len(path)-len(ext)]
	for i := 1; ; i++ {
		newPath := fmt.Sprintf("%s_%d%s", name, i, ext)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
	}
}
