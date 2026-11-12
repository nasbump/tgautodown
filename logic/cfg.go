package logic

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"tgautodown/cmd/tg"
	"tgautodown/internal/logs"
	"tgautodown/internal/utils"
)

type TgLogicConfig struct {
	AppID    int                `json:"appid"`
	AppHash  string             `json:"apphash"`
	Phone    string             `json:"phone"`
	CfgDir   string             `json:"cfgdir"`
	SaveDir  string             `json:"saveDir"`
	Gopeed   string             `json:"gopeed,omitempty"`
	HttpAddr string             `json:"httpaddr"`
	Channels []tg.ChannelConfig `json:"channels,omitempty"` // 订阅的频道/群组
	Proxy    string             `json:"proxy,omitempty"`    // socks5 代理地址
	F2A      string             `json:"f2a,omitempty"`      // 两步认证密码
	RetryCnt int                `json:"retrycnt,omitempty"`
	RetrySec int                `json:"retrysec,omitempty"`
	LogPath  string             `json:"logpath,omitempty"`
	LogLev   int                `json:"loglev,omitempty"`
	LogSize  int                `json:"logsize,omitempty"`
	LogCnt   int                `json:"logcnt,omitempty"`
	MaxDl    int                `json:"maxdl,omitempty"` // 同时下载任务数

	cfgPath     string `json:"-"`
	sessionPath string `json:"-"`
}

var TGCfg TgLogicConfig

var cfgMu sync.RWMutex

func ParseCfg() {
	TGCfg.CfgDir = "/app/data"
	TGCfg.SaveDir = "/app/download"
	TGCfg.Gopeed = "/app/bin/gopeed"
	TGCfg.HttpAddr = ":2020"
	TGCfg.Channels = []tg.ChannelConfig{}
	TGCfg.Proxy = "socks5://127.0.0.1:1080"
	TGCfg.RetryCnt = 10
	TGCfg.LogLev = 0
	TGCfg.LogSize = 300 << 20
	TGCfg.LogCnt = 1
	TGCfg.MaxDl = 1

	cfg := utils.XmArgValString("cfg", "config file path", TGCfg.CfgDir+"/config.json")
	utils.XmArgValBool("h", "show this help")
	utils.XmArgValBool("help", "same as -h")
	utils.XmUsageIfHasKeys("h", "help")

	if err := utils.XmCheckKnownArgs([]string{"cfg"}, []string{"h", "help"}); err != nil {
		log.Println(err)
		utils.XmUsage()
	}

	file, err := os.Open(cfg)
	if err != nil {
		log.Println("open", cfg, "fail")
	} else {
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&TGCfg); err != nil {
			log.Panic("load", cfg, "fail")
		}
		file.Close()
	}

	TGCfg.cfgPath = cfg
	TGCfg.sessionPath = filepath.Join(TGCfg.CfgDir, "session.json")
	TGCfg.Channels = normalizeChannels(TGCfg.Channels)
	TGCfg.RetryCnt = max(TGCfg.RetryCnt, 0)
	TGCfg.RetrySec = max(TGCfg.RetrySec, 0)
	TGCfg.MaxDl = max(TGCfg.MaxDl, 1)

	logs.LogsInit(TGCfg.LogPath, TGCfg.LogLev, TGCfg.LogSize, TGCfg.LogCnt)

	load, _ := json.Marshal(&TGCfg)
	logs.Info().Str("cfg", cfg).RawJSON("load", load).Send()
}

func normalizeChannels(channels []tg.ChannelConfig) []tg.ChannelConfig {
	res := make([]tg.ChannelConfig, 0, len(channels))
	for _, ch := range channels {
		ch.ID = strings.TrimSpace(ch.ID)
		ch.Name = strings.TrimSpace(ch.Name)
		ch.SaveDir = strings.TrimSpace(ch.SaveDir)
		if ch.SaveDir != "" {
			ch.SaveDir = filepath.Clean(ch.SaveDir)
		}
		if ch.ID == "" {
			continue
		}
		if ch.Name == "" {
			ch.Name = ch.ID
		}
		res = append(res, ch)
	}
	return res
}

// SetChannels 规范化并更新订阅源配置；仅修改内存与配置文件，需重连后才会重新解析。
func SetChannels(channels []tg.ChannelConfig) {
	res := normalizeChannels(channels)
	cfgMu.Lock()
	TGCfg.Channels = res
	cfgMu.Unlock()
}

// GetChannels 返回当前订阅源配置的副本。
func GetChannels() []tg.ChannelConfig {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return append([]tg.ChannelConfig(nil), TGCfg.Channels...)
}

func SaveCfg() {
	cfgsnap := func() TgLogicConfig {
		cfgMu.RLock()
		defer cfgMu.RUnlock()
		return TGCfg
	}()
	file, err := os.Create(cfgsnap.cfgPath)
	if err != nil {
		logs.Error(err).Str("cfg", cfgsnap.cfgPath).Msg("create fail")
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(&cfgsnap); err != nil {
		logs.Error(err).Str("cfg", cfgsnap.cfgPath).Msg("save fail")
	}
}
