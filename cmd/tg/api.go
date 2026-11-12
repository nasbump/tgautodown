package tg

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"tgautodown/internal/logs"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/proxy"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/tg"
)

const (
	TgstatusInit int = iota
	TgstatusLoging
	TgstatusLogOk
	TgstatusLogFail
)

var (
	ErrMsgClsUnsupport = errors.New("msgcls unsupport")
	ErrNoLoginCodeHnd  = errors.New("no login code handle")
	ErrNoF2APassword   = errors.New("no F2A passwd")
	ErrClientNotAuth   = errors.New("client not Authorized")
)

const (
	ChChannel int = iota
	ChGroup
	ChTelegramService
)

type SubChannelInfo struct {
	Name       string
	Title      string
	ChannelID  int64
	AccessHash int64
	Pts        int // 保存频道的 PTS（状态点）
	SlashCmd   bool
	SaveDir    string // 频道专属下载根目录，空表示使用全局 saveDir
	chType     int
}

// ChannelConfig 订阅源配置：Name 为可读名称，ID 为频道用户名或带 + 的邀请码。
type ChannelConfig struct {
	Name     string `json:"name"`
	ID       string `json:"id"`
	SlashCmd bool   `json:"slashcmd"`
	SaveDir  string `json:"saveDir,omitempty"` // 频道专属下载根目录，留空使用全局 saveDir
}

type TgSuber struct {
	AppID               int
	AppHash             string
	UserPhone           string
	SessionPath         string
	Socks5Proxy         string
	F2APassword         string
	FirstName, UserName string
	UserID, AccessHash  int64
	UserPwd             *proxy.Auth
	GetHistoryCnt       int
	MaxSaveRetryCnt     int    // 保存文件时最大重试次数
	MaxSaveRetryTime    int    // 保存文件时最大重试时长
	MaxDownload         int    // 同时下载任务数
	TaskStore           string // 未完成任务落盘路径

	enableTGLog  bool
	stateMu      sync.RWMutex
	client       *telegram.Client
	getLoginCode TgLoginCodeHnd
	scis         map[int64]SubChannelInfo
	mhnds        map[TgMsgClass]TgMsgHnd
	status       int
	gctx         context.Context
	gctxCancel   context.CancelFunc
	tasks        *TaskMgr
	tasksOnce    sync.Once
}

type TgMsgClass string
type TgMsgHnd func(int, *TgMsg) error
type TgLoginCodeHnd func() string
type TgOnSaveDoneHnd func(ts *TgSuber, savePath string, msgid int, tgmsg *TgMsg, err error)

type TgMsg struct {
	From     *SubChannelInfo
	Date     int64
	Text     string
	FileName string
	FileSize int64

	// TaskID 由任务管理器在入队/完成时填写
	TaskID int64

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
		AppID:       appid,
		AppHash:     apphash,
		UserPhone:   phone,
		mhnds:       map[TgMsgClass]TgMsgHnd{},
		status:      TgstatusInit,
		MaxDownload: 1,
	}
	return ts
}

func (ts *TgSuber) WithSession(path, f2apwd string, hnd TgLoginCodeHnd) *TgSuber {
	ts.SessionPath = path
	ts.F2APassword = f2apwd
	ts.getLoginCode = hnd
	return ts
}
func (ts *TgSuber) EnableTGLogger() *TgSuber {
	ts.enableTGLog = true
	return ts
}
func (ts *TgSuber) WithRetryRule(maxCnt, maxTime int) *TgSuber {
	ts.MaxSaveRetryCnt = maxCnt
	ts.MaxSaveRetryTime = maxTime
	return ts
}
func (ts *TgSuber) WithHistoryMsgCnt(cnt int) *TgSuber {
	ts.GetHistoryCnt = cnt
	return ts
}
func (ts *TgSuber) WithMaxDownload(n int) *TgSuber {
	if n < 1 {
		n = 1
	}
	ts.MaxDownload = n
	return ts
}
func (ts *TgSuber) WithTaskStore(path string) *TgSuber {
	ts.TaskStore = path
	return ts
}
func (ts *TgSuber) WithSocks5Proxy(addr string) *TgSuber {
	if addr == "" {
		return ts
	}

	u, err := url.Parse(addr)
	if err != nil {
		logs.Error(err).Str("url", addr).Msg("parse fail")
		return ts
	}
	ts.Socks5Proxy = u.Host
	if un := u.User.Username(); un != "" {
		ts.UserPwd = &proxy.Auth{
			User: un,
		}
		if p, ok := u.User.Password(); ok {
			ts.UserPwd.Password = p
		}
		logs.Info().Str("url", addr).Str("addr", ts.Socks5Proxy).
			Str("auth.user", ts.UserPwd.User).
			Str("auth.pwd", ts.UserPwd.Password).
			Msg("add proxy")
	} else {
		logs.Info().Str("url", addr).Str("addr", ts.Socks5Proxy).Msg("add proxy")
	}

	return ts
}

func (ts *TgSuber) WithMsgHandle(mcls TgMsgClass, hnd TgMsgHnd) *TgSuber {
	ts.mhnds[mcls] = hnd
	return ts
}

func (ts *TgSuber) Run(channels []ChannelConfig) error {
	logs.Info().Int("appid", ts.AppID).Str("apphash", ts.AppHash).Str("phone", ts.UserPhone).Str("socks5", ts.Socks5Proxy).Strs("channel", channelIDs(channels)).Send()

	ops := telegram.Options{}
	if ts.enableTGLog {
		ops.Logger, _ = zap.NewDevelopmentConfig().Build()
	}

	if ts.SessionPath != "" {
		ops.SessionStorage = &session.FileStorage{Path: ts.SessionPath}
	}

	if ts.Socks5Proxy != "" {
		socks5, err := proxy.SOCKS5("tcp", ts.Socks5Proxy, ts.UserPwd, proxy.Direct)
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

	ops.UpdateHandler = ts

	ts.taskMgr()

	for {
		gctx, gctxCancel := context.WithCancel(context.Background())
		client := telegram.NewClient(ts.AppID, ts.AppHash, ops)

		ts.stateMu.Lock()
		ts.gctx, ts.gctxCancel = gctx, gctxCancel
		ts.client = client
		ts.stateMu.Unlock()

		err := client.Run(gctx, func(ctx context.Context) error {
			return ts.run(ctx, channels)
		})

		ts.taskMgr().SetBaseCtx(nil)
		gctxCancel()

		ts.stateMu.Lock()
		ts.client = nil
		ts.stateMu.Unlock()

		logs.Warn(err).Msg("run fail, retry...")
		<-time.After(time.Second * 5)
	}
}

func channelIDs(channels []ChannelConfig) []string {
	ids := make([]string, 0, len(channels))
	for _, ch := range channels {
		ids = append(ids, ch.ID)
	}
	return ids
}

// taskMgr 在首次调用时创建任务管理器并恢复未完成任务。
func (ts *TgSuber) taskMgr() *TaskMgr {
	ts.tasksOnce.Do(func() {
		ts.tasks = newTaskMgr(ts, ts.TaskStore, ts.MaxDownload)
		ts.tasks.restore()
		ts.tasks.start()
	})
	return ts.tasks
}

// Tasks 返回任务管理器，供命令层使用。
func (ts *TgSuber) Tasks() *TaskMgr {
	return ts.taskMgr()
}

func (ts *TgSuber) api() *tg.Client {
	ts.stateMu.RLock()
	defer ts.stateMu.RUnlock()
	if ts.client == nil {
		return nil
	}
	return ts.client.API()
}

func (ts *TgSuber) clientRef() *telegram.Client {
	ts.stateMu.RLock()
	defer ts.stateMu.RUnlock()
	return ts.client
}

func (ts *TgSuber) connCtx() context.Context {
	ts.stateMu.RLock()
	defer ts.stateMu.RUnlock()
	return ts.gctx
}

func (ts *TgSuber) cancelConn() {
	ts.stateMu.RLock()
	cancel := ts.gctxCancel
	ts.stateMu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

func (ts *TgSuber) setStatus(status int) {
	ts.stateMu.Lock()
	ts.status = status
	ts.stateMu.Unlock()
}

func (ts *TgSuber) setScis(scis map[int64]SubChannelInfo) {
	ts.stateMu.Lock()
	ts.scis = scis
	ts.stateMu.Unlock()
}

func (ts *TgSuber) getSCI(id int64) (SubChannelInfo, bool) {
	ts.stateMu.RLock()
	defer ts.stateMu.RUnlock()
	sci, ok := ts.scis[id]
	return sci, ok
}

func (ts *TgSuber) ReplyTo(msg *TgMsg, text string) error {
	// return nil
	req := &tg.MessagesSendMessageRequest{
		Message:  text,
		RandomID: rand.Int63(), // 必须唯一
		ReplyTo: &tg.InputReplyToMessage{
			ReplyToMsgID: msg.msg.ID, // 你要回复的消息 ID
		},
	}
	switch msg.From.chType {
	case ChChannel:
		req.Peer = &tg.InputPeerChannel{
			ChannelID:  msg.From.ChannelID,
			AccessHash: msg.From.AccessHash,
		}
	case ChGroup:
		req.Peer = &tg.InputPeerChat{
			ChatID: msg.From.ChannelID,
		}
	default:
		logs.Warn(nil).Str("title", msg.From.Title).Msg("unknown channel or group")
		return nil
	}

	ctx := ts.connCtx()
	if ctx == nil {
		ctx = msg.ctx
	}
	api := ts.api()
	if ctx == nil || api == nil {
		return ErrClientNotAuth
	}
	if _, err := api.MessagesSendMessage(ctx, req); err != nil {
		logs.Warn(err).Str("title", msg.From.Title).Msg("reply fail")
		return err
	}

	return nil
}

// SaveFile 将文件下载任务加入队列，返回任务 ID。
func (ts *TgSuber) SaveFile(msg *TgMsg, savePath string, done TgOnSaveDoneHnd) (int64, error) {
	switch msg.mcls {
	case TgPhoto, TgVideo, TgAudio, TgDocument:
		return ts.taskMgr().add(msg, savePath, done), nil
	default:
		return 0, ErrMsgClsUnsupport
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
	// // 如果结果为空，就用时间戳兜底
	// if name == "" {
	// 	name = fmt.Sprintf("file_%d", time.Now().Unix())
	// }
	return name
}

func (ts *TgSuber) Status() int {
	ts.stateMu.RLock()
	defer ts.stateMu.RUnlock()
	return ts.status
}
