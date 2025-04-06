package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type tgAutoDownload struct {
	token                  string
	saveDir                string
	gopeed                 string
	botSrv, botApiEndpoint string
	debug                  bool
	botApi                 *tgbot.BotAPI
}

var tgAD tgAutoDownload

func main() {
	tgAD.parseArgs()
	tgAD.newBotApi()
	tgAD.login()
	tgAD.poll()
}

func (tg *tgAutoDownload) newBotApi() {
	var tgapi *tgbot.BotAPI
	var err error

	if 1 == 0 {
		tgapi, err = tgbot.NewBotAPI(tg.token)
		if err != nil {
			log.Panic("NewBotAPI", err)
		}
		if tg.botSrv != "" {
			tg.switchApiEndpoint()
		}
	} else {
		if tg.botSrv != "" {
			tgapi, err = tgbot.NewBotAPIWithAPIEndpoint(tg.token, tg.botApiEndpoint)
		} else {
			tgapi, err = tgbot.NewBotAPI(tg.token)
		}

		if err != nil {
			log.Panic("NewBotAPI", err)
		}
	}

	tg.botApi = tgapi
	tgapi.Debug = tg.debug
}

func (tg *tgAutoDownload) switchApiEndpoint() error {
	resp, err := tg.botApi.Request(tgbot.LogOutConfig{})
	if err != nil {
		return err
	}

	log.Printf("logOut.ok:%t, result:%s", resp.Ok, string(resp.Result))
	tg.botApi.SetAPIEndpoint(tg.botApiEndpoint)
	return nil
}

func (tg *tgAutoDownload) login() {
	var delay time.Duration = 1
	for {
		user, err := tg.botApi.GetMe()
		if err != nil {
			log.Printf("login fail: %v, delay=%ds\n", err, delay)
			time.Sleep(delay * time.Second)
			delay <<= 1
			if delay > 120 {
				delay = 120
			}
			continue
		}

		log.Printf("login succ: %+v", user)
		return
	}
}

func (tg *tgAutoDownload) poll() {
	updateConfig := tgbot.NewUpdate(0)
	updateConfig.Timeout = 30

	updates := tg.botApi.GetUpdatesChan(updateConfig)
	for update := range updates {
		if update.Message == nil {
			continue
		}

		rMsg := update.Message
		msgID := strconv.Itoa(update.Message.MessageID)

		log.Printf("recv updateID: %d, msgID: %s, from: %v at %d",
			update.UpdateID, msgID, rMsg.From, rMsg.Date)

		switch {
		case rMsg.Video != nil:
			tg.reply(update, "正在下载视频, 消息ID: "+msgID)
			fileID := update.Message.Video.FileID
			go tg.download("videos", fileID, msgID, update)
		case rMsg.Audio != nil:
			tg.reply(update, "正在下载音频, 消息ID: "+msgID)
			fileID := update.Message.Audio.FileID
			go tg.download("music", fileID, msgID, update)
		case rMsg.Document != nil:
			tg.reply(update, "正在下载文档, 消息ID: "+msgID)
			fileID := update.Message.Document.FileID
			go tg.download("documents", fileID, msgID, update)
		case rMsg.Photo != nil:
			tg.reply(update, "正在下载照片, 消息ID: "+msgID)
			go tg.downloadPhotos("photos", msgID, update)
		case len(rMsg.Text) > 0:
			if strings.HasPrefix(strings.ToLower(rMsg.Text), "magnet:?") {
				tg.reply(update, "正在下载BT, 消息ID: "+msgID)
				go tg.downloadMagnet("bt", msgID, update)
			} else {
				tg.writeNote("note", update)
			}
		default:
			tg.reply(update, "你发的是啥啊？")
		}
	}
}

func (tg *tgAutoDownload) download(dlType, fileID, msgID string, update tgbot.Update) error {
	if fp, err := tg.doDownload(fileID, dlType); err != nil {
		replyMsg := "下载失败:" + dlType
		replyMsg += "\n- 消息ID: " + msgID
		replyMsg += "\n- 失败信息: " + err.Error()
		tg.reply(update, replyMsg)
		return err
	} else {
		replyMsg := "下载成功:" + dlType
		replyMsg += "\n- 消息ID: " + msgID
		replyMsg += "\n- 文件大小: " + sizeInt2Readable(fp.FileSize)
		replyMsg += "\n- 保存路径: " + fp.FilePath
		tg.reply(update, replyMsg)
		return nil
	}
}

func (tg *tgAutoDownload) downloadPhotos(dlType, msgID string, update tgbot.Update) {
	succ := 0
	fail := 0

	for _, photo := range update.Message.Photo {
		if _, err := tg.doDownload(photo.FileID, dlType); err != nil {
			fail++
		} else {
			succ++
		}
	}

	replyMsg := "下载完成:" + dlType
	replyMsg += "\n- 消息ID: " + msgID
	replyMsg += "\n- 保存路径: " + filepath.Join(tg.saveDir, dlType)
	replyMsg += fmt.Sprintf("\n- 成功：%d, 失败：%d", succ, fail)
	tg.reply(update, replyMsg)
}
func (tg *tgAutoDownload) downloadMagnet(dlType, msgID string, update tgbot.Update) {
	url := update.Message.Text
	log.Println(url)

	savePath := filepath.Join(tg.saveDir, dlType, msgID)
	if err := createDir(savePath); err != nil {
		replyMsg := "BT创建失败:"
		replyMsg += "\n- 消息ID: " + msgID
		replyMsg += "\n- 失败信息: " + err.Error()
		tg.reply(update, replyMsg)
		return
	}

	if err := exec.Command(tg.gopeed, "-C", "32", "-D", savePath, url).Run(); err != nil {
		replyMsg := "BT下载失败:"
		replyMsg += "\n- 消息ID: " + msgID
		replyMsg += "\n- 失败信息: " + err.Error()
		tg.reply(update, replyMsg)
	} else {
		tg.reply(update, "BT下载完成: "+savePath)

		replyMsg := "BT下载成功:"
		replyMsg += "\n- 消息ID: " + msgID
		replyMsg += "\n- 保存路径: " + savePath + "/"
		tg.reply(update, replyMsg)
	}
}

func (tg *tgAutoDownload) doDownload(fileID, dlType string) (tgbot.File, error) {
	if tg.botSrv != "" {
		/*
			如果你 运行自己的 Bot API 服务器，则 file_path 将是指向本地磁盘上文件的绝对文件路径。
			在这种情况下，你无需再下载任何内容，因为 Bot API 服务器会在调用 getFile 时为你下载文件。
		*/
		return tg.doDownloadByBotApi(fileID)
	}
	return tg.doDownloadByDirect(fileID, dlType)
}

func (tg *tgAutoDownload) doDownloadByBotApi(fileID string) (tgbot.File, error) {
	fp, err := tg.botApi.GetFile(tgbot.FileConfig{FileID: fileID})
	if err != nil {
		return fp, err
	}

	return fp, nil
}
func (tg *tgAutoDownload) doDownloadByDirect(fileID, dlType string) (tgbot.File, error) {
	fp, err := tg.botApi.GetFile(tgbot.FileConfig{FileID: fileID})
	if err != nil {
		return fp, err
	}

	url := fp.Link(tg.token)
	log.Println(url)

	savePath := filepath.Join(tg.saveDir, dlType)
	if err := createDir(savePath); err != nil {
		return fp, err
	}

	// todo: 每个url都只有60分钟有效时间，没下载完的话，需要重新调用getfile以获取最新链接
	fp.FilePath = filepath.Join(savePath, filepath.Base(fp.FilePath))
	return fp, exec.Command(tg.gopeed, "-C", "6", "-D", savePath, url).Run()
}

func (tg *tgAutoDownload) writeNote(dlType string, update tgbot.Update) {
	note := update.Message.Text

	savePath := filepath.Join(tg.saveDir, dlType)
	if err := createDir(savePath); err != nil {
		tg.reply(update, "添加笔记失败: "+err.Error())
		return
	}
	savePath = filepath.Join(savePath, strconv.Itoa(update.Message.MessageID)+".md")
	if err := os.WriteFile(savePath, []byte(note), 0666); err != nil {
		tg.reply(update, "添加笔记失败: "+err.Error())
	} else {
		tg.reply(update, "添加笔记成功: "+savePath)
	}
}

func (tg *tgAutoDownload) reply(update tgbot.Update, replyTxt string) {
	msg := tgbot.NewMessage(update.Message.Chat.ID, replyTxt)
	msg.ReplyToMessageID = update.Message.MessageID
	if _, err := tg.botApi.Send(msg); err != nil {
		log.Printf("reply msg:%s to %d fail:%v\n", replyTxt, msg.ReplyToMessageID, err)
	}
}

func (tg *tgAutoDownload) parseArgs() {
	flag.StringVar(&tg.token, "token", "", "tg-bot-token")
	flag.StringVar(&tg.saveDir, "dir", "./", "save dir")
	flag.StringVar(&tg.gopeed, "gopeed", "", "gopeed path")
	flag.BoolVar(&tg.debug, "debug", false, "enable bot-api debug")
	flag.StringVar(&tg.botSrv, "botSrv", "", "bot-api-server: http://localhost:8081")
	flag.Parse()

	if tg.token == "" {
		if tg.token = os.Getenv("TGBOT_TOKEN"); tg.token == "" {
			log.Panic("tg-bot-token miss")
		}
	}

	tg.saveDir = filepath.Join(tg.saveDir, tg.token)

	if tg.botSrv != "" {
		tg.botApiEndpoint = tg.botSrv + "/bot%s/%s"
		//tg.botFileEndpoint = tg.botSrv + "/file/bot%s/%s"
		log.Println("tg.botApiEndpoint", tg.botApiEndpoint)
		//log.Println("tg.botFileEndpoint", tg.botFileEndpoint)
		//log.Println("tgbot.APIEndpoint", tgbot.APIEndpoint)
		//log.Println("tgbot.FileEndpoint", tgbot.FileEndpoint)
	}
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

func sizeInt2Readable(size int) string {
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
