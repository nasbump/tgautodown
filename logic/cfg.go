package logic

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"tgautodown/internal/logs"
	"tgautodown/internal/utils"
)

type TgLogicConfig struct {
	AppID    int    `json:"appid"`
	AppHash  string `json:"apphash"`
	Phone    string `json:"phone"`
	CfgDir   string `json:"cfgdir"`
	SaveDir  string `json:"saveDir"`
	Gopeed   string `json:"gopeed,omitempty"`
	HttpAddr string `json:"httpaddr"`
	LogPath  string `json:"logpath,omitempty"`
	LogLev   int    `json:"loglev,omitempty"`
	LogSize  int    `json:"logsize,omitempty"`
	LogCnt   int    `json:"logcnt,omitempty"`

	channelNames       []string `json:"-"`
	socks5             string   `json:"-"`
	f2apwd             string   `json:"-"`
	cfgPath            string   `json:"-"`
	sessionPath        string   `json:"-"`
	maxSaveRetryCnt    int      `json:"-"`
	maxSaveRetrySecond int      `json:"-"`
}

var TGCfg TgLogicConfig

func ParseCfg() {
	TGCfg.CfgDir = "/app/data"
	TGCfg.SaveDir = "/app/download"
	TGCfg.Gopeed = "/app/bin/gopeed"
	TGCfg.HttpAddr = ":2020"
	TGCfg.LogLev = 0
	TGCfg.LogSize = 300 << 20
	TGCfg.LogCnt = 1

	TGCfg.channelNames = utils.XmArgValStrings("names", "channels name", "")
	TGCfg.socks5 = utils.XmArgValString("proxy", "socks5://127.0.0.1:1080", "")
	TGCfg.maxSaveRetrySecond = utils.XmArgValInt("retrysec", "max retry seconds", 10*60)
	TGCfg.maxSaveRetryCnt = utils.XmArgValInt("retrycnt", "max retry count", 10)
	TGCfg.f2apwd = utils.XmArgValString("f2a", "password for F2A", "")

	cfg := utils.XmArgValString("cfg", "", TGCfg.CfgDir+"/config.json")

	utils.XmUsageIfHasKeys("h", "help")

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

	TGCfg.cfgPath = filepath.Join(TGCfg.CfgDir, "config.json")
	TGCfg.sessionPath = filepath.Join(TGCfg.CfgDir, "session.json")

	logs.LogsInit(TGCfg.LogPath, TGCfg.LogLev, TGCfg.LogSize, TGCfg.LogCnt)

	load, _ := json.Marshal(&TGCfg)
	logs.Info().Str("cfg", cfg).RawJSON("load", load).Send()
}

func SaveCfg() {
	file, err := os.Create(TGCfg.cfgPath)
	if err != nil {
		logs.Error(err).Str("cfg", TGCfg.cfgPath).Msg("create fail")
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(&TGCfg); err != nil {
		logs.Error(err).Str("cfg", TGCfg.cfgPath).Msg("save fail")
	}
}
