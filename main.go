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

	tgapi, err = tgbot.NewBotAPI(tg.token)
	if err != nil {
		log.Panic(err)
	}
	tgapi.Debug = tg.debug

	// if tg.botSrv != "" {
	// 	tgapi, err = tgbot.NewBotAPIWithAPIEndpoint(tg.token, tg.botApiEndpoint)
	// } else {
	// 	tgapi, err = tgbot.NewBotAPI(tg.token)
	// }

	// if err != nil {
	// 	log.Panic(err)
	// }
	// tgapi.Debug = tg.debug

	tg.botApi = tgapi

	if tg.botSrv != "" {
		tg.switchApiEndpoint()
	}
}

func (tg *tgAutoDownload) switchApiEndpoint() error {
	//resp, err := tg.botApi.Request(tgbot.LogOutConfig{})
	//if err != nil {
	//	return err
	//}

	//log.Printf("logOut.ok:%t, result:%s", resp.Ok, string(resp.Result))
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

		log.Printf("recv updateID: %d, msgID: %d, from: %v at %d",
			update.UpdateID, rMsg.MessageID, rMsg.From, rMsg.Date)

		switch {
		case rMsg.Video != nil:
			go tg.downloadVideo(update)
			tg.reply(update, "收到视频，正在下载!")
		case rMsg.Audio != nil:
			go tg.downloadAudio(update)
			tg.reply(update, "收到音频，正在下载!")
		case rMsg.Document != nil:
			go tg.downloadDocument(update)
			tg.reply(update, "收到文档，正在下载!")
		case rMsg.Photo != nil:
			go tg.downloadPhotos(update)
			tg.reply(update, "收到照片，正在下载!")
		case len(rMsg.Text) > 0:
			if strings.HasPrefix(strings.ToLower(rMsg.Text), "magnet:?") {
				go tg.downloadMagnet(update)
				tg.reply(update, "收到磁力链接，正在下载!")
			} else {
				tg.writeNote(update)
			}
		default:
			tg.reply(update, "你发的是啥啊？")
		}
	}
}

func (tg *tgAutoDownload) downloadVideo(update tgbot.Update) {
	fileID := update.Message.Video.FileID
	savePath := filepath.Join(tg.saveDir, "video", strconv.Itoa(update.Message.MessageID))

	if dlpath, err := tg.download(fileID, savePath); err != nil {
		tg.reply(update, "下载失败: "+err.Error())
	} else {
		tg.reply(update, "下载完成: "+dlpath)
	}
}
func (tg *tgAutoDownload) downloadAudio(update tgbot.Update) {
	fileID := update.Message.Audio.FileID
	savePath := filepath.Join(tg.saveDir, "audio", strconv.Itoa(update.Message.MessageID))

	if dlpath, err := tg.download(fileID, savePath); err != nil {
		tg.reply(update, "下载失败: "+err.Error())
	} else {
		tg.reply(update, "下载完成: "+dlpath)
	}
}
func (tg *tgAutoDownload) downloadDocument(update tgbot.Update) {
	fileID := update.Message.Document.FileID
	savePath := filepath.Join(tg.saveDir, "doc", strconv.Itoa(update.Message.MessageID))

	if dlpath, err := tg.download(fileID, savePath); err != nil {
		tg.reply(update, "下载失败: "+err.Error())
	} else {
		tg.reply(update, "下载完成: "+dlpath)
	}
}

func (tg *tgAutoDownload) downloadPhotos(update tgbot.Update) {
	succ := 0
	fail := 0
	savePath := filepath.Join(tg.saveDir, "photos", strconv.Itoa(update.Message.MessageID))

	for _, photo := range update.Message.Photo {
		if _, err := tg.download(photo.FileID, savePath); err != nil {
			fail++
		} else {
			succ++
		}
	}

	replyTxt := fmt.Sprintf("下载完成， 成功：%d, 失败：%d", succ, fail)
	tg.reply(update, replyTxt)
}
func (tg *tgAutoDownload) downloadMagnet(update tgbot.Update) {
	url := update.Message.Text
	log.Println(url)

	savePath := filepath.Join(tg.saveDir, "bt", strconv.Itoa(update.Message.MessageID))
	if err := createDir(savePath); err != nil {
		tg.reply(update, "下载失败: "+err.Error())
		return
	}

	if err := exec.Command(tg.gopeed, "-C", "32", "-D", savePath, url).Run(); err != nil {
		tg.reply(update, "下载失败: "+err.Error())
	} else {
		tg.reply(update, "下载完成: "+savePath)
	}
}

func (tg *tgAutoDownload) download(fileID, savePath string) (string, error) {

	if tg.botSrv != "" {
		/*
			如果你 运行自己的 Bot API 服务器，则 file_path 将是指向本地磁盘上文件的绝对文件路径。
			在这种情况下，你无需再下载任何内容，因为 Bot API 服务器会在调用 getFile 时为你下载文件。
		*/
		return tg.downloadByBotApi(fileID)
	}
	return savePath, tg.downloadByDirect(fileID, savePath)
}

func (tg *tgAutoDownload) downloadByBotApi(fileID string) (string, error) {
	fp, err := tg.botApi.GetFile(tgbot.FileConfig{FileID: fileID})
	if err != nil {
		return "", err
	}

	return fp.FilePath, nil
}
func (tg *tgAutoDownload) downloadByDirect(fileID, savePath string) error {
	url, err := tg.botApi.GetFileDirectURL(fileID)
	if err != nil {
		return err
	}

	log.Println(url)
	if err := createDir(savePath); err != nil {
		return err
	}

	// todo: 每个url都只有60分钟有效时间，没下载完的话，需要重新调用getfile以获取最新链接
	return exec.Command(tg.gopeed, "-C", "6", "-D", savePath, url).Run()
}

func (tg *tgAutoDownload) writeNote(update tgbot.Update) {
	note := update.Message.Text

	savePath := filepath.Join(tg.saveDir, "note")
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
		if tg.token = os.Getenv("TGBOTOKEN"); tg.token == "" {
			log.Panic("tg-bot-token miss")
		}
	}

	if tg.botSrv != "" {
		tg.saveDir = filepath.Join(tg.saveDir, tg.token)
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
