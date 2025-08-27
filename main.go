package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"tgautodown/cmd/tg"
	"tgautodown/internal/logs"
	"tgautodown/internal/utils"
)

var namesMap = map[tg.TgMsgClass][]string{
	tg.TgVideo:    {"videos", "视频"},
	tg.TgAudio:    {"music", "音乐"},
	tg.TgDocument: {"documents", "文档"},
	tg.TgPhoto:    {"photos", "照片"},
	tg.TgNote:     {"note", "笔记"},
	"bt":          {"bt", "BT"},
}

type tgAutoDownload struct {
	appID         int
	appHash       string
	phone         string
	socks5        string
	channelNames  []string
	sessionPath   string
	getHistoryCnt int

	// token   string
	saveDir string
	gopeed  string
	botSrv  string
}

func main() {
	var tgAD tgAutoDownload

	tgAD.appID = utils.XmArgValInt("appid", "https://core.telegram.org/api/obtaining_api_id", 0)
	tgAD.appHash = utils.XmArgValString("apphash", "", "")
	tgAD.phone = utils.XmArgValString("phone", "your login phone number", "")
	tgAD.channelNames = utils.XmArgValStrings("names", "channel names", "")
	tgAD.sessionPath = utils.XmArgValString("session", "session file", "./session.json")
	tgAD.getHistoryCnt = utils.XmArgValInt("history", "get history msg count", 0)
	tgAD.socks5 = utils.XmArgValString("proxy", "proxy url: socks5://127.0.0.1:1080", "")

	// tgAD.token = utils.XmArgValString("token", "tg-bot-token", "")
	tgAD.saveDir = utils.XmArgValString("dir", "save dir", "./")
	// tgAD.botSrv = utils.XmArgValString("botSrv", "bot-api-server: http://localhost:8081", "")
	tgAD.gopeed = utils.XmArgValString("gopeed", "gopeed path", "")
	utils.XmLogsInit("", 0, 50<<20, 1) // 设置日志级别为0(DEBUG)

	utils.XmUsageIfHasKeys("h", "help")
	utils.XmUsageIfHasNoKeys("appid", "apphash")

	ts := tg.NewTG(tgAD.appID, tgAD.appHash, tgAD.phone).
		WithHistoryMsgCnt(tgAD.getHistoryCnt).
		WithSocks5Proxy(tgAD.socks5)

	ts.WithMsgHandle(tg.TgAudio, func(msgid int, tgmsg *tg.TgMsg) error {
		return tgAD.doDownload(ts, tg.TgAudio, msgid, tgmsg)
	})
	ts.WithMsgHandle(tg.TgDocument, func(msgid int, tgmsg *tg.TgMsg) error {
		return tgAD.doDownload(ts, tg.TgDocument, msgid, tgmsg)
	})
	ts.WithMsgHandle(tg.TgVideo, func(msgid int, tgmsg *tg.TgMsg) error {
		return tgAD.doDownload(ts, tg.TgVideo, msgid, tgmsg)
	})
	ts.WithMsgHandle(tg.TgPhoto, func(msgid int, tgmsg *tg.TgMsg) error {
		return tgAD.doDownload(ts, tg.TgPhoto, msgid, tgmsg)
	})
	ts.WithMsgHandle(tg.TgNote, func(msgid int, tgmsg *tg.TgMsg) error {
		if strings.HasPrefix(strings.ToLower(tgmsg.Text), "magnet:?") {
			return tgAD.downloadMagnet(ts, "bt", msgid, tgmsg)
		} else {
			return tgAD.writeNote(ts, tg.TgNote, msgid, tgmsg)
		}
	})

	ts.WithSession(tgAD.sessionPath, func() string {
		return tgAD.inputLoginCode()
	}).Run(tgAD.channelNames)
}

func (tgAD *tgAutoDownload) inputLoginCode() string {
	for {
		// 从标准输入读取验证码
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("请输入收到的验证码: ")
		code, err := reader.ReadString('\n')
		if err != nil {
			// logs.Error(err).Msg("读取验证码失败")
			continue
		}
		code = strings.TrimSpace(code)
		if code != "" {
			return code
		}
	}
}

func (tgAD *tgAutoDownload) doDownload(ts *tg.TgSuber, mtype tg.TgMsgClass, msgid int, tgmsg *tg.TgMsg) error {
	subDir := namesMap[mtype][0]
	mtDesc := namesMap[mtype][1]

	replyMsg := fmt.Sprintf("正在下载%s: %s\n- 文件大小: %s\n- 消息ID: %d",
		mtDesc, tgmsg.FileName, sizeInt2Readable(tgmsg.FileSize), msgid)
	logs.Debug().Msg(replyMsg)
	ts.ReplyTo(tgmsg, replyMsg)
	savePath := tgAD.getSavePath(subDir, tgmsg.FileName)
	if err := ts.SaveFile(tgmsg, savePath); err != nil {
		replyMsg = fmt.Sprintf("下载失败: %s\n- 消息ID: %d\n- 失败原因: %s",
			tgmsg.FileName, msgid, err.Error())
	} else {
		replyMsg = fmt.Sprintf("下载成功: %s\n- 消息ID: %d\n- 保存路径: %s",
			tgmsg.FileName, msgid, savePath)
	}
	logs.Debug().Msg(replyMsg)
	return ts.ReplyTo(tgmsg, replyMsg)
}

func (tgAD *tgAutoDownload) getSavePath(mtype, filename string) string {
	savePath := filepath.Join(tgAD.saveDir, mtype)
	createDir(savePath)
	savePath = filepath.Join(savePath, filename)
	return uniquePath(savePath)
}

func (tgAD *tgAutoDownload) downloadMagnet(ts *tg.TgSuber, mtype tg.TgMsgClass, msgid int, tgmsg *tg.TgMsg) error {
	subDir := namesMap[mtype][0]
	mtDesc := namesMap[mtype][1]
	url := tgmsg.Text
	logs.Debug().Int("msgid", msgid).Str("url", url).Msg("recv magnet")

	replyMsg := fmt.Sprintf("正在下载%s:\n- 消息ID: %d", mtDesc, msgid)
	ts.ReplyTo(tgmsg, replyMsg)

	savePath := filepath.Join(tgAD.saveDir, subDir)
	createDir(savePath)

	err := exec.Command(tgAD.gopeed, "-C", "32", "-D", savePath, url).Run()
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

func (tgAD *tgAutoDownload) writeNote(ts *tg.TgSuber, mtype tg.TgMsgClass, msgid int, tgmsg *tg.TgMsg) error {
	subDir := namesMap[mtype][0]
	mtDesc := namesMap[mtype][1]
	note := tgmsg.Text
	logs.Debug().Int("msgid", msgid).Str("note", note).Msg("recv note")

	savePath := filepath.Join(tgAD.saveDir, subDir)
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
