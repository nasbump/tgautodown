package tg

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"tgautodown/internal/logs"
	"time"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

func (ts *TgSuber) handle(ctx context.Context, names []string) error {
	ts.status = TgstatusLoging
	if err := ts.login(ctx); err != nil {
		ts.status = TgstatusLogFail
		return err
	}

	// 获取当前用户信息，拿到 self ID
	self, err := ts.client.Self(ctx)
	if err != nil {
		ts.status = TgstatusLogFail
		logs.Warn(err).Msg("get self fail")
		return err
	}

	ts.status = TgstatusLogOk
	ts.FirstName = self.FirstName
	ts.UserName = self.Username
	logs.Info().Str("firstname", self.FirstName).Str("username", self.Username).
		Int64("id", self.ID).Int64("accesshash", self.AccessHash).
		Bool("bot", self.Bot).Str("phone", ts.Phone).
		Int("appid", ts.AppID).Msg("ready")

	cs := ts.getChannels(ctx, names)
	if len(cs) == 0 {
		logs.Error(nil).Msg("no channels need subscribe")
		return nil
	}

	wg := sync.WaitGroup{}
	wg.Add(len(cs))
	for _, sci := range cs {
		go func() {
			if ts.GetHistoryCnt > 0 {
				ts.recvChannelHistoryMsg(ctx, &sci)
			}
			ts.recvChannelDiffMsg(ctx, &sci)
			wg.Done()
		}()
	}

	wg.Wait()
	return nil
}

func (ts *TgSuber) login(ctx context.Context) error {
	tsca := ts.client.Auth()
	// 尝试从会话恢复
	status, err := tsca.Status(ctx)
	if err != nil {
		logs.Warn(err).Msg("client.Auth.Status fail")
		return err
	}

	if status.Authorized {
		return nil
	}

	if ts.getLoginCode == nil {
		logs.Warn(ErrNoLoginCodeHnd).Send()
		return ErrNoLoginCodeHnd
	}

	// 发送验证码请求
	sentCode, err := tsca.SendCode(ctx, ts.Phone, auth.SendCodeOptions{})
	if err != nil {
		logs.Error(err).Msg("client.SendCode fail")
		return err
	}
	// 类型断言获取PhoneCodeHash
	codeSent, ok := sentCode.(*tg.AuthSentCode)
	if !ok {
		logs.Error(nil).Msg("invalid tg.AuthSentCode")
		return err
	}

	for {
		code := ts.getLoginCode()
		if code == "" {
			continue
		}

		// 验证登录
		if _, err := tsca.SignIn(ctx, ts.Phone, code, codeSent.PhoneCodeHash); err != nil {
			logs.Error(err).Str("code", code).Msg("signin fail")
			continue
		}
		break
	}

	return nil
}

func (ts *TgSuber) getChannels(ctx context.Context, names []string) map[int64]SubChannelInfo {
	cs := map[int64]SubChannelInfo{}

	api := ts.client.API()
	for _, name := range names {
		if strings.HasPrefix(name, "+") { // https://t.me/+sZF0XrTZVq02M2Yx
			name = strings.TrimPrefix(name, "+")
			invite, err := api.MessagesCheckChatInvite(ctx, name)
			if err != nil {
				logs.Warn(err).Str("name", name).Msg("check invite fail")
				continue
			}
			switch inv := invite.(type) {
			case *tg.ChatInviteAlready:
				if ch, ok := inv.Chat.(*tg.Channel); ok {
					sci := SubChannelInfo{
						Name:       name, // ch.Username, // 私有频道没有username
						Title:      ch.Title,
						ChannelID:  ch.ID,
						AccessHash: ch.AccessHash,
					}
					cs[ch.ID] = sci
					logs.Info().Str("name", name).Int64("id", sci.ChannelID).Int64("hash", sci.AccessHash).Str("title", sci.Title).Msg("private")
				}
			case *tg.ChatInvite: // 未加入，需要调用 MessagesImportChatInvite 加入
				// joined, _ := api.MessagesImportChatInvite(ctx, name)
				logs.Warn(nil).Str("name", name).Msg("not in channel")
			}
		} else {
			res, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
				Username: name,
			})
			if err != nil {
				logs.Warn(err).Str("channel.name", name).Msg("resolve fail")
				continue
			}

			if ch, ok := res.Chats[0].(*tg.Channel); ok {
				sci := SubChannelInfo{
					Name:       ch.Username,
					Title:      ch.Title,
					ChannelID:  ch.ID,
					AccessHash: ch.AccessHash,
				}

				cs[ch.ID] = sci
				logs.Info().Str("name", name).Int64("id", sci.ChannelID).Int64("hash", sci.AccessHash).Str("title", sci.Title).Msg("public")
			}
		}
	}
	return cs
}

func (ts *TgSuber) recvChannelHistoryMsg(ctx context.Context, sci *SubChannelInfo) {
	api := ts.client.API()

	peer := &tg.InputPeerChannel{
		ChannelID:  sci.ChannelID,
		AccessHash: sci.AccessHash,
	}

	// 建立上下文（必须要调一次，不然不会推送消息）
	_, err := api.ChannelsGetFullChannel(ctx, &tg.InputChannel{
		ChannelID:  peer.ChannelID,
		AccessHash: peer.AccessHash,
	})
	if err != nil {
		logs.Error(err).Str("channel", sci.Name).Str("title", sci.Title).Msg("ChannelsGetFullChannel fail")
		return
	}

	history, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:  peer,
		Limit: ts.GetHistoryCnt,
	})
	if err != nil {
		logs.Warn(err).Str("channel", sci.Name).Str("title", sci.Title).Msg("MessagesGetHistory fail")
	}

	msgs, ok := history.(*tg.MessagesChannelMessages)
	if !ok {
		logs.Debug().Str("channel", sci.Name).Str("title", sci.Title).Msgf("recv non-channel-msg:%+v", msgs)
		return
	}

	logs.Debug().Int("msgs.Messages.size", len(msgs.Messages))
	for _, m := range msgs.Messages {
		if msg, ok := m.(*tg.Message); ok {
			ts.recvChannelMsgHandle(ctx, msg, sci)
		}
	}
}

func (ts *TgSuber) recvChannelDiffMsg(ctx context.Context, sci *SubChannelInfo) {
	api := ts.client.API()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	peer := &tg.InputPeerChannel{
		ChannelID:  sci.ChannelID,
		AccessHash: sci.AccessHash,
	}

	// 建立上下文（必须要调一次，不然不会推送消息）
	full, err := api.ChannelsGetFullChannel(ctx, &tg.InputChannel{
		ChannelID:  peer.ChannelID,
		AccessHash: peer.AccessHash,
	})
	if err != nil {
		logs.Error(err).Str("channel", sci.Name).Str("title", sci.Title).Msg("ChannelsGetFullChannel fail")
		return
	}

	chatFull, ok := full.FullChat.(*tg.ChannelFull)
	if !ok {
		logs.Error(err).Str("channel", sci.Name).Str("title", sci.Title).Msg("FullChat fail")
		return
	}

	pts := chatFull.Pts

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			diff, err := api.UpdatesGetChannelDifference(ctx, &tg.UpdatesGetChannelDifferenceRequest{
				Channel: &tg.InputChannel{
					ChannelID:  peer.ChannelID,
					AccessHash: peer.AccessHash,
				},
				Filter: &tg.ChannelMessagesFilterEmpty{},
				Pts:    pts,
				Limit:  50,
			})
			if err != nil {
				logs.Warn(err).Str("channel", sci.Name).Str("title", sci.Title).Msg("UpdatesGetChannelDifference fail")
				continue
			}

			switch upd := diff.(type) {
			case *tg.UpdatesChannelDifference:
				for _, m := range upd.NewMessages {
					if msg, ok := m.(*tg.Message); ok {
						ts.recvChannelMsgHandle(ctx, msg, sci)
					}
				}
				// 更新 PTS
				pts = upd.Pts
			case *tg.UpdatesChannelDifferenceEmpty:
				// 没有新消息，更新 PTS
				pts = upd.Pts
			}
		}
	}
}

func (ts *TgSuber) recvChannelMsgHandle(ctx context.Context, msg *tg.Message, sci *SubChannelInfo) error {
	if msg.ReplyTo != nil {
		logs.Trace().Msg("skip reply.msg")
		return nil
	}

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
		FileName: fmt.Sprintf("%s_%d.jpg", sci.Name, photo.Date),
		FileSize: int64(maxSize),

		mcls:  TgPhoto,
		ptype: ptype,
		msg:   msg,
		ctx:   ctx,
	}
	return ts.mhnds[TgPhoto](msg.ID, &tgmsg)
}

func (ts *TgSuber) savePhoto(ctx context.Context, tgmsg *TgMsg, savePath string) error {
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

	return ts.fileSaveLocation(ctx, tgmsg.FileSize, savePath, location)
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
		FileSize: int64(doc.GetSize()),

		msg: msg,
		ctx: ctx,
	}

	switch {
	case strings.HasPrefix(doc.MimeType, "video/"):
		tgmsg.mcls = TgVideo
		tgmsg.FileName = fmt.Sprintf("%s_%d.mp4", sci.Name, doc.Date)
	case strings.HasPrefix(doc.MimeType, "audio/"):
		tgmsg.mcls = TgAudio
		tgmsg.FileName = fmt.Sprintf("%s_%d.mp3", sci.Name, doc.Date)
	default:
		tgmsg.mcls = TgDocument
		tgmsg.FileName = fmt.Sprintf("%s_%d.pdf", sci.Name, doc.Date)
		logs.Debug().Str("media", media.String()).Send()
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

func (ts *TgSuber) saveMedia(ctx context.Context, tgmsg *TgMsg, savePath string) error {
	msg := tgmsg.msg
	media := msg.Media.(*tg.MessageMediaDocument)
	doc := media.Document.(*tg.Document)

	// 构造下载位置
	location := &tg.InputDocumentFileLocation{
		ID:            doc.ID,
		AccessHash:    doc.AccessHash,
		FileReference: doc.FileReference,
	}

	return ts.fileSaveLocation(ctx, tgmsg.FileSize, savePath, location)
}

func (ts *TgSuber) fileSaveLocation(ctx context.Context, filesize int64, filename string, location tg.InputFileLocationClass) error {
	// 打开本地文件
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	// 分块下载
	const chunkSize = 512 * 1024 // 512 KB
	offset := int64(0)

	api := ts.client.API()
	for {
		// 请求文件分片
		part, err := api.UploadGetFile(ctx, &tg.UploadGetFileRequest{
			Location: location,
			Offset:   offset,
			Limit:    chunkSize,
		})
		if err != nil {
			return fmt.Errorf("get file part: %w", err)
		}

		// 判断类型
		switch v := part.(type) {
		case *tg.UploadFile:
			// 写入数据
			vsize := len(v.Bytes)
			wsize, err := file.Write(v.Bytes)
			if err != nil || vsize != wsize {
				logs.Warn(err).Int("vsize", vsize).Int("wsize", wsize).Msg("write file fail")
				return fmt.Errorf("write file: %w", err)
			}

			offset += int64(wsize)

			logs.Debug().Str("file", filename).Str("dl.progress", calcDlProgress(offset, filesize)).Send()

			// 如果不足 chunkSize 说明结束
			if wsize < chunkSize {
				logs.Info().Int64("dlsize", offset).Str("filename", filename).Msg("dl succ")
				return nil
			}
		default:
			return fmt.Errorf("unexpected type %T", v)
		}
	}
}

func calcDlProgress(dl, tot int64) string {
	percent := float64(dl) * 100 / float64(tot)
	return fmt.Sprintf("%d/%d=%.2f", dl, tot, percent)
}
