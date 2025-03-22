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
	token   string
	saveDir string
	gopeed  string
	botApi  *tgbot.BotAPI
}

var tgAD tgAutoDownload

func main() {
	tgAD.parseArgs()
	tgAD.newBotApi()
	tgAD.login()
	tgAD.poll()
}

func (tg *tgAutoDownload) newBotApi() {
	tgapi, err := tgbot.NewBotAPI(tg.token)
	if err != nil {
		log.Panic(err)
	}
	tgapi.Debug = false
	tg.botApi = tgapi
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

	if err := tg.download(fileID, savePath); err != nil {
		tg.reply(update, "下载失败: "+err.Error())
	} else {
		tg.reply(update, "下载完成: "+savePath)
	}
}
func (tg *tgAutoDownload) downloadAudio(update tgbot.Update) {
	fileID := update.Message.Audio.FileID
	savePath := filepath.Join(tg.saveDir, "audio", strconv.Itoa(update.Message.MessageID))

	if err := tg.download(fileID, savePath); err != nil {
		tg.reply(update, "下载失败: "+err.Error())
	} else {
		tg.reply(update, "下载完成: "+savePath)
	}
}
func (tg *tgAutoDownload) downloadDocument(update tgbot.Update) {
	fileID := update.Message.Document.FileID
	savePath := filepath.Join(tg.saveDir, "doc", strconv.Itoa(update.Message.MessageID))

	if err := tg.download(fileID, savePath); err != nil {
		tg.reply(update, "下载失败: "+err.Error())
	} else {
		tg.reply(update, "下载完成: "+savePath)
	}
}

func (tg *tgAutoDownload) downloadPhotos(update tgbot.Update) {
	succ := 0
	fail := 0
	savePath := filepath.Join(tg.saveDir, "photos", strconv.Itoa(update.Message.MessageID))

	for _, photo := range update.Message.Photo {
		if err := tg.download(photo.FileID, savePath); err != nil {
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

func (tg *tgAutoDownload) download(fileID, savePath string) error {
	url, err := tg.botApi.GetFileDirectURL(fileID)
	if err != nil {
		return err
	}

	log.Println(url)

	if err := createDir(savePath); err != nil {
		return err
	}

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
		tg.reply(update, "写笔记失败: "+err.Error())
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
	flag.Parse()

	if tg.token == "" {
		if tg.token = os.Getenv("TGBOTOKEN"); tg.token == "" {
			log.Panic("tg-bot-token miss")
		}
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
