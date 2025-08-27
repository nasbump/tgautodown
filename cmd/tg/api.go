package tg

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"regexp"
	"strings"
	"tgautodown/internal/logs"
	"time"

	"golang.org/x/net/proxy"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/tg"
)

var (
	ErrMsgClsUnsupport = errors.New("msgcls unsupport")
	ErrNoLoginCodeHnd  = errors.New("no login code handle")
)

type SubChannelInfo struct {
	Name       string
	Title      string
	ChannelID  int64
	AccessHash int64
	Pts        int // 保存频道的 PTS（状态点）
}

type TgSuber struct {
	AppID         int
	AppHash       string
	Phone         string
	SessionPath   string
	Socks5Proxy   string
	GetHistoryCnt int

	client       *telegram.Client
	getLoginCode TgLoginCodeHnd
	mhnds        map[TgMsgClass]TgMsgHnd
}

type TgMsgClass string
type TgMsgHnd func(int, *TgMsg) error
type TgLoginCodeHnd func() string

type TgMsg struct {
	From     *SubChannelInfo
	Text     string
	FileName string
	FileSize int64

	ctx   context.Context
	msg   *tg.Message
	mcls  TgMsgClass
	ptype string // for photo
}

const (
	TgVideo    TgMsgClass = "vedio"
	TgAudio    TgMsgClass = "music"
	TgDocument TgMsgClass = "document"
	TgPhoto    TgMsgClass = "photo"
	TgNote     TgMsgClass = "note"
)

func NewTG(appid int, apphash, phone string) *TgSuber {
	ts := &TgSuber{
		AppID:   appid,
		AppHash: apphash,
		Phone:   phone,
		mhnds:   map[TgMsgClass]TgMsgHnd{},
	}
	return ts
}

func (ts *TgSuber) WithSession(path string, hnd TgLoginCodeHnd) *TgSuber {
	ts.SessionPath = path
	ts.getLoginCode = hnd
	return ts
}

func (ts *TgSuber) WithHistoryMsgCnt(cnt int) *TgSuber {
	ts.GetHistoryCnt = cnt
	return ts
}
func (ts *TgSuber) WithSocks5Proxy(addr string) *TgSuber {
	ts.Socks5Proxy = addr
	return ts
}

func (ts *TgSuber) WithMsgHandle(mcls TgMsgClass, hnd TgMsgHnd) *TgSuber {
	ts.mhnds[mcls] = hnd
	return ts
}

func (ts *TgSuber) Run(names []string) error {
	logs.Info().Int("appid", ts.AppID).Str("apphash", ts.AppHash).Str("socks5", ts.Socks5Proxy).Strs("channel", names).Send()

	// zlog, _ := zap.NewDevelopmentConfig().Build()

	ops := telegram.Options{
		// Logger: zlog,
	}

	if ts.SessionPath != "" {
		ops.SessionStorage = &session.FileStorage{Path: ts.SessionPath}
	}

	if ts.Socks5Proxy != "" {
		socks5, err := proxy.SOCKS5("tcp", ts.Socks5Proxy, nil, proxy.Direct)
		if err != nil {
			logs.Warn(err).Str("socks5", ts.Socks5Proxy).Msg("create proxy fail")
			return err
		}

		var dial dcs.DialFunc
		if dc, ok := socks5.(proxy.ContextDialer); ok {
			dial = dc.DialContext
		} else {
			dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return socks5.Dial(network, addr)
			}
		}

		// 2. 创建 Telegram 客户端并指定代理
		ops.Resolver = dcs.Plain(dcs.PlainOptions{
			Dial: dial,
		})
		ops.DialTimeout = 15 * time.Second
	}

	ts.client = telegram.NewClient(ts.AppID, ts.AppHash, ops)

	return ts.client.Run(context.Background(), func(ctx context.Context) error {
		return ts.handle(ctx, names)
	})
}

func (ts *TgSuber) ReplyTo(msg *TgMsg, text string) error {
	_, err := ts.client.API().MessagesSendMessage(msg.ctx, &tg.MessagesSendMessageRequest{
		Peer: &tg.InputPeerChannel{
			ChannelID:  msg.From.ChannelID,
			AccessHash: msg.From.AccessHash,
		},
		Message:  text,
		RandomID: rand.Int63(), // 必须唯一，可用 rand.Int63()
		ReplyTo: &tg.InputReplyToMessage{
			ReplyToMsgID: msg.msg.ID, // 你要回复的消息 ID
		},
	})
	return err
}

func (ts *TgSuber) SaveFile(msg *TgMsg, savePath string) error {
	switch msg.mcls {
	case TgPhoto:
		return ts.savePhoto(msg.ctx, msg, savePath)
	case TgVideo:
		return ts.saveMedia(msg.ctx, msg, savePath)
	case TgAudio:
		return ts.saveMedia(msg.ctx, msg, savePath)
	case TgDocument:
		return ts.saveMedia(msg.ctx, msg, savePath)
	default:
		return ErrMsgClsUnsupport
	}
}

// 清理非法文件名字符
func sanitizeFileName(name string) string {
	// 去掉开头结尾空格
	name = strings.TrimSpace(name)
	// 空格替换成 "_"
	name = strings.ReplaceAll(name, " ", "_")
	// 去掉 Windows/Linux 不允许的字符
	re := regexp.MustCompile(`[\\/:*?"<>|]+`)
	name = re.ReplaceAllString(name, "_")
	// 如果结果为空，就用时间戳兜底
	if name == "" {
		name = fmt.Sprintf("file_%d", time.Now().Unix())
	}
	return name
}
