package tg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"tgautodown/internal/logs"
	"time"

	"github.com/gotd/td/rpc"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

func (ts *TgSuber) run(ctx context.Context, names []string) error {
	ts.status = TgstatusLoging
	if err := ts.login(ctx); err != nil {
		ts.status = TgstatusLogFail
		return err
	}

	ts.status = TgstatusLogOk
	logs.Info().Str("firstname", ts.FirstName).Str("username", ts.UserName).
		Int64("id", ts.UserID).Int64("accesshash", ts.AccessHash).
		Str("phone", ts.UserPhone).Int("appid", ts.AppID).Msg("ready")

	cs := ts.getChannels(ctx, names)
	if len(cs) == 0 {
		logs.Error(nil).Msg("no channels need subscribe")
		return nil
	}
	ts.scis = cs

	if ts.GetHistoryCnt > 0 {
		for _, sci := range cs {
			ts.recvHistoryMsg(ctx, &sci)
		}
	}

	<-ts.gctx.Done()
	return ts.gctx.Err()
}

func (ts *TgSuber) login(ctx context.Context) error {
	flow := auth.NewFlow(ts, auth.SendCodeOptions{})

	tsca := ts.client.Auth()

	if err := tsca.IfNecessary(ctx, flow); err != nil {
		logs.Error(err).Msg("login failed")
		return err
	}

	self, err := ts.client.Self(ctx)
	if err != nil {
		if auth.IsUnauthorized(err) {
			logs.Fatal(err).Msg("ErrClientNotAuth")
			return ErrClientNotAuth
		}
		logs.Warn(err).Msg("client.Auth.Status fail")
		return err
	}

	ts.FirstName = self.FirstName
	ts.UserName = self.Username
	ts.AccessHash = self.AccessHash
	ts.UserID = self.ID

	return nil
}

func (ts *TgSuber) getChannels(ctx context.Context, names []string) map[int64]SubChannelInfo {
	cs := map[int64]SubChannelInfo{}

	api := ts.client.API()
	for _, name := range names {
		if after, ok := strings.CutPrefix(name, "+"); ok { // https://t.me/+sZF0XrTZVq02M2Yx
			invite, err := api.MessagesCheckChatInvite(ctx, after)
			if err != nil {
				logs.Warn(err).Str("name", name).Msg("check invite fail")
				continue
			}
			switch inv := invite.(type) {
			case *tg.ChatInviteAlready:
				if ch, ok := inv.Chat.(*tg.Channel); ok {
					sci := SubChannelInfo{
						Name:       after, // ch.Username, // 私有频道没有username
						Title:      ch.Title,
						ChannelID:  ch.ID,
						AccessHash: ch.AccessHash,
						chType:     ChChannel,
					}
					cs[ch.ID] = sci
					logs.Info().Str("name", name).Int64("id", sci.ChannelID).Int64("hash", sci.AccessHash).Str("title", sci.Title).Msg("private channel")
				} else if ch, ok := inv.Chat.(*tg.Chat); ok {
					sci := SubChannelInfo{
						Name:      after, // ch.Username, // 私有频道没有username
						Title:     ch.Title,
						ChannelID: ch.ID,
						chType:    ChGroup,
					}
					cs[ch.ID] = sci
					logs.Info().Str("name", name).Int64("id", sci.ChannelID).Str("title", sci.Title).Msg("private group")
				} else {
					logs.Warn(nil).Str("name", name).Str("invite.type", inv.TypeName()).
						Str("chat.type", inv.Chat.TypeName()).Msg("unknown chat")
				}
			case *tg.ChatInvite: // 未加入，需要调用 MessagesImportChatInvite 加入
				// joined, _ := api.MessagesImportChatInvite(ctx, name)
				logs.Warn(nil).Str("name", name).Str("invite.type", inv.TypeName()).
					Bool("Channel", inv.Channel).
					Bool("Public", inv.Public).Msg("not in channel")
			default:
				logs.Warn(nil).Str("name", name).Str("invite.type", inv.TypeName()).Msg("unknown invite")
			}
		} else {
			res, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
				Username: name,
			})
			if err != nil {
				logs.Warn(err).Str("name", name).Msg("resolve fail")
				continue
			}

			inv := res.Chats[0]
			if ch, ok := inv.(*tg.Channel); ok {
				sci := SubChannelInfo{
					Name:       ch.Username,
					Title:      ch.Title,
					ChannelID:  ch.ID,
					AccessHash: ch.AccessHash,
					chType:     ChChannel,
				}

				cs[ch.ID] = sci
				logs.Info().Str("name", name).Int64("id", sci.ChannelID).Int64("hash", sci.AccessHash).Str("title", sci.Title).Msg("public chanel")
			} else if ch, ok := inv.(*tg.Chat); ok {
				sci := SubChannelInfo{
					Name:      name,
					Title:     ch.Title,
					ChannelID: ch.ID,
					chType:    ChGroup,
				}
				cs[ch.ID] = sci
				logs.Info().Str("name", name).Int64("id", sci.ChannelID).Str("title", sci.Title).Msg("public group")
			} else {
				logs.Warn(nil).Str("name", name).Str("invite.type", inv.TypeName()).Msg("unknown invite")
			}
		}
	}
	return cs
}

func (ts *TgSuber) recvHistoryMsg(ctx context.Context, sci *SubChannelInfo) {
	api := ts.client.API()

	req := &tg.MessagesGetHistoryRequest{
		Limit: ts.GetHistoryCnt,
	}

	switch sci.chType {
	case ChChannel:
		req.Peer = &tg.InputPeerChannel{
			ChannelID:  sci.ChannelID,
			AccessHash: sci.AccessHash,
		}
	case ChGroup:
		req.Peer = &tg.InputPeerChat{
			ChatID: sci.ChannelID,
		}
	default:
		logs.Warn(nil).Str("title", sci.Title).Msg("unknown channel or group")
		return
	}

	history, err := api.MessagesGetHistory(ctx, req)
	if err != nil {
		logs.Warn(err).Str("group", sci.Name).Str("title", sci.Title).Msg("MessagesGetHistory fail")
		return
	}

	switch msgs := history.(type) {
	case *tg.MessagesMessages:
		logs.Debug().Int("msgs.Messages.size", len(msgs.Messages)).Send()
		for _, m := range msgs.Messages {
			if msg, ok := m.(*tg.Message); ok {
				ts.recvChannelMsgHandle(ctx, msg, sci)
			}
		}
	case *tg.MessagesChannelMessages:
		logs.Debug().Int("msgs.Messages.size", len(msgs.Messages)).Send()
		for _, m := range msgs.Messages {
			if msg, ok := m.(*tg.Message); ok {
				ts.recvChannelMsgHandle(ctx, msg, sci)
			}
		}
	case *tg.MessagesMessagesSlice:
		logs.Debug().Int("msgs.Messages.size", len(msgs.Messages)).Send()
		for _, m := range msgs.Messages {
			if msg, ok := m.(*tg.Message); ok {
				ts.recvChannelMsgHandle(ctx, msg, sci)
			}
		}
	default:
		logs.Warn(nil).Str("group", sci.Name).Str("history", msgs.TypeName()).Msg("unknown history")
	}
}

func (ts *TgSuber) refreshMsg(ctx context.Context, tgmsg *TgMsg) tg.InputFileLocationClass {
	api := ts.client.API()

	fresh, err := api.MessagesGetMessages(ctx, []tg.InputMessageClass{&tg.InputMessageID{ID: tgmsg.msg.ID}})
	if err != nil {
		logs.Warn(err).Str("channel", tgmsg.From.Name).Int("msgid", tgmsg.msg.ID).Msg("refresh fail")
		return nil
	}

	switch msgs := fresh.(type) {
	case *tg.MessagesMessages:
		if newMsg, ok := msgs.Messages[0].(*tg.Message); ok {
			tgmsg.msg = newMsg
		}
	case *tg.MessagesChannelMessages:
		if newMsg, ok := msgs.Messages[0].(*tg.Message); ok {
			tgmsg.msg = newMsg
		}
	case *tg.MessagesMessagesSlice:
		if newMsg, ok := msgs.Messages[0].(*tg.Message); ok {
			tgmsg.msg = newMsg
		}
	default:
		logs.Warn(nil).Str("channel", tgmsg.From.Name).Int("msgid", tgmsg.msg.ID).Str("fresh", msgs.TypeName()).Msg("unknown fresh")
		return nil

	}

	switch tgmsg.mcls {
	case TgPhoto:
		msg := tgmsg.msg
		media := msg.Media.(*tg.MessageMediaPhoto)
		photo := media.Photo.(*tg.Photo)
		return &tg.InputPhotoFileLocation{
			ID:            photo.ID,
			AccessHash:    photo.AccessHash,
			FileReference: photo.FileReference,
			ThumbSize:     tgmsg.ptype, // 可选缩略图大小 ("s", "m", "x", "y", "w", "z" 等)
		}
	default:
		msg := tgmsg.msg
		media := msg.Media.(*tg.MessageMediaDocument)
		doc := media.Document.(*tg.Document)
		return &tg.InputDocumentFileLocation{
			ID:            doc.ID,
			AccessHash:    doc.AccessHash,
			FileReference: doc.FileReference,
		}
	}
}

func (ts *TgSuber) recvChannelMsgHandle(ctx context.Context, msg *tg.Message, sci *SubChannelInfo) error {
	if msg.ReplyTo != nil {
		logs.Trace().Int("msgid", msg.ID).Msg("skip reply.msg")
		return nil
	}

	logs.Debug().Int("msgid", msg.ID).Str("from", sci.Title).Msg("on recv msg")

	if msg.Media == nil { // 接收到文本消息
		return ts.recvChannelNoteMsg(ctx, msg, sci)
	}
	// else 接收到视频/音频/文档/照片消息
	if _, ok := msg.Media.(*tg.MessageMediaPhoto); ok {
		return ts.recvChannelPhotoMsg(ctx, msg, sci)
	}
	if _, ok := msg.Media.(*tg.MessageMediaDocument); ok { // 视频/音频/文档
		return ts.recvChannelMediaMsg(ctx, msg, sci)
	}
	// 其他消息不支持
	logs.Warn(ErrMsgClsUnsupport).Int("msgid", msg.ID).Str("from", sci.Title).Msg("recv unknown msg")
	return ErrMsgClsUnsupport
}
func (ts *TgSuber) recvChannelNoteMsg(ctx context.Context, msg *tg.Message, sci *SubChannelInfo) error {
	if msg.Message == "" {
		logs.Trace().Int("msgid", msg.ID).Msg("blank")
		return nil
	}

	if ts.mhnds[TgNote] == nil {
		logs.Trace().Int("msgid", msg.ID).Msg("no note.handler")
		return nil
	}

	tgmsg := TgMsg{
		From: sci,
		Text: msg.Message,
		Date: int64(msg.Date),

		mcls: TgNote,
		msg:  msg,
		ctx:  ctx,
	}
	return ts.mhnds[TgNote](msg.ID, &tgmsg)
}

func (ts *TgSuber) recvChannelPhotoMsg(ctx context.Context, msg *tg.Message, sci *SubChannelInfo) error {
	if ts.mhnds[TgPhoto] == nil {
		logs.Trace().Int("msgid", msg.ID).Msg("no photo.handler")
		return nil
	}

	media := msg.Media.(*tg.MessageMediaPhoto)
	photo, ok := media.Photo.(*tg.Photo)
	if !ok {
		logs.Trace().Int("msgid", msg.ID).Msg("not photo msg")
		return nil
	}

	ptype := "x"
	maxSize := 0

	for _, size := range photo.Sizes {
		switch s := size.(type) {
		case *tg.PhotoSize:
			if s.Size > maxSize {
				maxSize = s.Size
				ptype = s.Type
			}
			logs.Trace().Str("type", s.Type).Int("w", s.W).Int("h", s.H).Int("filesize", s.Size).Str("ptype", ptype).Msg("PhotoSize")
		case *tg.PhotoCachedSize:
			size := len(s.Bytes)
			if size > maxSize {
				maxSize = size
				ptype = s.Type
			}
			logs.Trace().Str("type", s.Type).Int("w", s.W).Int("h", s.H).Int("filesize", size).Str("ptype", ptype).Msg("PhotoCachedSize")
		}
	}

	tgmsg := TgMsg{
		From:     sci,
		Text:     msg.Message,
		FileName: fmt.Sprintf("%s_%d_%d.jpg", sci.Name, photo.Date, msg.ID),
		FileSize: int64(maxSize),
		Date:     int64(msg.Date),

		mcls:  TgPhoto,
		ptype: ptype,
		msg:   msg,
		ctx:   ctx,
	}
	return ts.mhnds[TgPhoto](msg.ID, &tgmsg)
}

func (ts *TgSuber) savePhoto(ctx context.Context, tgmsg *TgMsg, savePath string, done TgOnSaveDoneHnd) error {
	msg := tgmsg.msg
	// sci := tgmsg.From

	media := msg.Media.(*tg.MessageMediaPhoto)
	photo := media.Photo.(*tg.Photo)

	location := &tg.InputPhotoFileLocation{
		ID:            photo.ID,
		AccessHash:    photo.AccessHash,
		FileReference: photo.FileReference,
		ThumbSize:     tgmsg.ptype, // 可选缩略图大小 ("s", "m", "x", "y", "w", "z" 等)
	}

	return ts.fileSaveLocation(ctx, tgmsg, savePath, location, done)
}

func (ts *TgSuber) recvChannelMediaMsg(ctx context.Context, msg *tg.Message, sci *SubChannelInfo) error {
	media := msg.Media.(*tg.MessageMediaDocument)
	doc, ok := media.Document.(*tg.Document)
	if !ok {
		logs.Trace().Int("msgid", msg.ID).Msg("not media msg")
		return nil
	}

	tgmsg := TgMsg{
		From:     sci,
		Text:     msg.Message,
		FileSize: int64(doc.GetSize()),
		Date:     int64(msg.Date),

		msg: msg,
		ctx: ctx,
	}

	txt := sanitizeFileName(msg.Message)
	if len(msg.Message) > 255 {
		txt = tgmsg.Text[:255]
	}

	switch {
	case strings.HasPrefix(doc.MimeType, "video/"):
		tgmsg.mcls = TgVideo
		if len(txt) > 0 {
			tgmsg.FileName = txt + ".mp4"
		} else {
			tgmsg.FileName = fmt.Sprintf("%s_%d.mp4", sci.Name, doc.Date)
		}
	case strings.HasPrefix(doc.MimeType, "audio/"):
		tgmsg.mcls = TgAudio
		if len(txt) > 0 {
			tgmsg.FileName = txt + ".mp3"
		} else {
			tgmsg.FileName = fmt.Sprintf("%s_%d.mp3", sci.Name, doc.Date)
		}
	default:
		tgmsg.mcls = TgDocument
		if len(txt) > 0 {
			tgmsg.FileName = txt + ".pdf"
		} else {
			tgmsg.FileName = fmt.Sprintf("%s_%d.pdf", sci.Name, doc.Date)
		}
		// logs.Debug().Str("media", media.String()).Send()
	}

	if ts.mhnds[tgmsg.mcls] == nil {
		logs.Trace().Int("msgid", msg.ID).Str("mcls", string(tgmsg.mcls)).Msg("no media.handler")
		return nil
	}

	for _, attr := range doc.Attributes {
		if attrName, ok := attr.(*tg.DocumentAttributeFilename); ok {
			tgmsg.FileName = sanitizeFileName(attrName.FileName)
			logs.Debug().Str("attrName.FileName", tgmsg.FileName).Msg("get attr.name")
			break
		}
	}

	return ts.mhnds[tgmsg.mcls](msg.ID, &tgmsg)
}

func (ts *TgSuber) saveMedia(ctx context.Context, tgmsg *TgMsg, savePath string, done TgOnSaveDoneHnd) error {
	msg := tgmsg.msg
	media := msg.Media.(*tg.MessageMediaDocument)
	doc := media.Document.(*tg.Document)

	// 构造下载位置
	location := &tg.InputDocumentFileLocation{
		ID:            doc.ID,
		AccessHash:    doc.AccessHash,
		FileReference: doc.FileReference,
	}

	return ts.fileSaveLocation(ctx, tgmsg, savePath, location, done)
}

func resetSaveRetry() (int, time.Time) {
	return 0, time.Now()
}
func (ts *TgSuber) checkSaveRetry(cnt int, beg time.Time) bool {
	if ts.MaxSaveRetryCnt > 0 && cnt > ts.MaxSaveRetryCnt {
		return false

	}
	if ts.MaxSaveRetryTime > 0 {
		if cost := time.Since(beg).Milliseconds(); cost > int64(ts.MaxSaveRetryTime)*1000 {
			return false
		}
	}
	return true
}

type fileSaveMsg struct {
	ctx      context.Context
	tgmsg    *TgMsg
	filename string
	location tg.InputFileLocationClass
	done     TgOnSaveDoneHnd
	retryCnt int
}

var fileSaveChan = make(chan *fileSaveMsg, 200)

func (ts *TgSuber) fileSaveLocation(ctx context.Context, tgmsg *TgMsg, filename string, location tg.InputFileLocationClass, done TgOnSaveDoneHnd) error {
	fsm := &fileSaveMsg{
		ctx:      ctx,
		tgmsg:    tgmsg,
		filename: filename,
		location: location,
		done:     done,
	}

	fileSaveChan <- fsm
	return nil
}

func (ts *TgSuber) fileSaveLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			logs.Debug().Msg("fileSaveLoop exit")
			return
		case fsm := <-fileSaveChan:
			err := ts.doFileSaveLocation(fsm.ctx, fsm.tgmsg, fsm.filename, fsm.location)
			if err != nil {
				if fsm.retryCnt+1 > ts.MaxSaveRetryCnt {
					logs.Error(err).Int("msgid", fsm.tgmsg.msg.ID).Str("filename", fsm.filename).
						Int("retry", fsm.retryCnt).Msg("too many retry times")
					if fsm.done != nil {
						fsm.done(ts, fsm.filename, fsm.tgmsg.msg.ID, fsm.tgmsg, err)
					}
					continue
				}
				nfsm := &fileSaveMsg{
					ctx:      context.Background(),
					tgmsg:    fsm.tgmsg,
					filename: fsm.filename,
					location: fsm.location,
					done:     fsm.done,
					retryCnt: fsm.retryCnt + 1,
				}
				fileSaveChan <- nfsm
			} else {
				logs.Trace().Int("msgid", fsm.tgmsg.msg.ID).Str("filename", fsm.filename).Msg("file save ok")
				if fsm.done != nil {
					fsm.done(ts, fsm.filename, fsm.tgmsg.msg.ID, fsm.tgmsg, nil)
				}
			}
		}
	}
}

func (ts *TgSuber) doFileSaveLocation(ctx context.Context, tgmsg *TgMsg, filename string, location tg.InputFileLocationClass) error {
	// 检查文件是否已经存在且完整
	if finfo, err := os.Stat(filename); err == nil {
		if finfo.Size() >= tgmsg.FileSize {
			logs.Debug().Str("file", filename).Int("msgid", tgmsg.msg.ID).Int64("filesize", tgmsg.FileSize).Msg("file already exists and complete")
			return nil
		}
	}

	dlname := filename + ".dl"
	if finfo, err := os.Stat(dlname); err == nil {
		if finfo.Size() >= tgmsg.FileSize {
			logs.Debug().Str("file", filename).Int("msgid", tgmsg.msg.ID).Int64("filesize", tgmsg.FileSize).Msg("file already exists and complete")
			return os.Rename(dlname, filename)
		}
	}

	file, err := os.OpenFile(dlname, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	// 分块下载
	const chunkSize = 64 << 10
	offset := int64(0)
	if finfo, err := file.Stat(); err == nil {
		offset = finfo.Size()
	}

	api := ts.client.API()
	filesize := tgmsg.FileSize
	retryCnt := 0
	retryBeg := time.Now()

	logs.Debug().Str("file", filename).Int("msgid", tgmsg.msg.ID).Int64("filesize", filesize).Msg("save beg")

	for {
		// 请求文件分片
		part, err := api.UploadGetFile(ctx, &tg.UploadGetFileRequest{
			Location: location,
			Offset:   offset,
			Limit:    chunkSize,
		})
		if err != nil {
			logs.Warn(err).Str("file", filename).Int64("offset", offset).Int64("filesize", filesize).
				Int("retry", retryCnt).Msg("get file part fail")

			if tg.IsFileReferenceExpired(err) {
				if location = ts.refreshMsg(ctx, tgmsg); location == nil {
					return err
				}
				continue
			}
			if errors.Is(err, rpc.ErrEngineClosed) {
				ts.gctxCancel() // 返回并重新登陆
				return err
			}

			if retry := ts.checkSaveRetry(retryCnt, retryBeg); retry {
				retryCnt++
				continue
			}

			if strings.Contains(err.Error(), "intermediate") {
				ts.gctxCancel()
				return rpc.ErrEngineClosed
			}
			return err
		}
		// 成功则重置
		retryCnt, retryBeg = resetSaveRetry()

		// 判断类型
		switch v := part.(type) {
		case *tg.UploadFile:
			// 写入数据
			vsize := len(v.Bytes)
			wsize, err := file.Write(v.Bytes)
			if err != nil || vsize != wsize {
				logs.Warn(err).Str("file", filename).Int("vsize", vsize).Int("wsize", wsize).Msg("write file fail")
				return fmt.Errorf("write file: %w", err)
			}

			offset += int64(wsize)

			logs.Debug().Str("file", filename).Str("dl.progress", calcDlProgress(offset, filesize)).Send()

			// 如果不足 chunkSize 说明结束
			if wsize < chunkSize || offset >= filesize {
				logs.Info().Int64("dlsize", offset).Int64("filesize", filesize).Str("filename", filename).Msg("dl succ")
				return os.Rename(dlname, filename)
			}
		default:
			return fmt.Errorf("unexpected type %T", v)
		}
	}
}

func calcDlProgress(dl, tot int64) string {
	percent := float64(dl) * 100 / float64(tot)
	return fmt.Sprintf("%d/%d=%.2f%%", dl, tot, percent)
}
