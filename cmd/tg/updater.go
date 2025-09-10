package tg

import (
	"context"
	"tgautodown/internal/logs"

	"github.com/gotd/td/tg"
)

func (ts *TgSuber) Handle(ctx context.Context, u tg.UpdatesClass) error {
	switch upd := u.(type) {
	case *tg.Updates:
		// 常见容器：Updates 包含多个 UpdateClass
		for _, up := range upd.Updates {
			ts.handleSingleUpdate(ctx, up)
		}
	case *tg.UpdatesCombined:
		// UpdatesCombined 同样包含 Updates 列表
		for _, up := range upd.Updates {
			ts.handleSingleUpdate(ctx, up)
		}
	case *tg.UpdatesTooLong:
		// 服务器提示更新太多，需要从 getDialogs/getHistory 或者 updates.getDifference 拉取
		logs.Warn(nil).Msg("on UpdatesTooLong, need to resync or fetch dialogs/history")
	case *tg.UpdateShort:
		// UpdateShort 包含一个内嵌的 UpdateClass（可能性较少），尝试向下断言
		if inner := upd.Update; inner != nil {
			ts.handleSingleUpdate(ctx, inner)
		}
	default:
		// 其他容器类型，打印以便调试
		logs.Warn(nil).Str("upd", upd.TypeName()).Msg("unknown update class")
	}
	return nil
}

func (ts *TgSuber) handleSingleUpdate(ctx context.Context, up tg.UpdateClass) {
	switch u := up.(type) {
	case *tg.UpdateNewMessage:
		if msg, ok := u.Message.(*tg.Message); ok {
			if sci := ts.getSubChannel(msg.PeerID); sci != nil {
				ts.recvChannelMsgHandle(ctx, msg, sci)
			}
		}
	case *tg.UpdateNewChannelMessage:
		if msg, ok := u.Message.(*tg.Message); ok {
			if sci := ts.getSubChannel(msg.PeerID); sci != nil {
				ts.recvChannelMsgHandle(ctx, msg, sci)
			}
		}
	case *tg.UpdateUserStatus:
		logs.Trace().Int64("userid", u.UserID).Str("status", u.Status.String()).Msg("on UpdateUserStatus")
	case *tg.UpdateEditChannelMessage:
		if msg, ok := u.Message.(*tg.Message); ok {
			logs.Trace().Int("msgid", msg.ID).Str("msg", msg.Message).Msg("on UpdateEditChannelMessage")
		} else {
			logs.Trace().Str("msg", u.Message.TypeName()).Msg("on UpdateEditChannelMessage")
		}
	case *tg.UpdateChannelMessageViews:
		logs.Trace().Str("msg", u.String()).Msg("on UpdateChannelMessageViews")
	case *tg.UpdateReadChannelInbox:
		logs.Trace().Str("msg", u.String()).Msg("on UpdateReadChannelInbox")
	case *tg.UpdateReadHistoryOutbox:
		logs.Trace().Str("msg", u.String()).Msg("on UpdateReadHistoryOutbox")
	case *tg.UpdateDraftMessage:
		logs.Trace().Str("msg", u.String()).Msg("on UpdateDraftMessage")
	default:
		logs.Trace().Str("update", u.TypeName()).Msg("un-support updater")
	}
}

func (ts *TgSuber) getSubChannel(peer tg.PeerClass) *SubChannelInfo {
	var chid int64 = 0
	switch p := peer.(type) {
	case *tg.PeerChat:
		chid = p.ChatID
	case *tg.PeerChannel:
		chid = p.ChannelID
	default:
		logs.Warn(nil).Str("peer", peer.TypeName()).Msg("unknown peer")
		return nil
	}

	if ts.scis != nil {
		if sci, ok := ts.scis[chid]; ok {
			return &sci
		}
	}
	return nil
}
