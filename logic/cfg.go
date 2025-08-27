package logic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"tgautodown/internal/logs"
	"tgautodown/internal/utils"
)

type TgLogicConfig struct {
	AppID        int      `json:"appid"`
	AppHash      string   `json:"apphash"`
	Phone        string   `json:"phone"`
	Socks5       string   `json:"proxy"`
	ChannelNames []string `json:"names,omitempty"`
	CfgDir       string   `json:"cfgdir"`
	SaveDir      string   `json:"saveDir"`
	Gopeed       string   `json:"gopeed,omitempty"`
	HttpAddr     string   `json:"httpaddr"`
	LogPath      string   `json:"logpath,omitempty"`
	LogLev       int      `json:"loglev,omitempty"`
	LogSize      int      `json:"logsize,omitempty"`
	LogCnt       int      `json:"logcnt,omitempty"`

	cfgPath     string `json:"-"`
	sessionPath string `json:"-"`
}

var TGCfg TgLogicConfig

func ParseCfg() {
	TGCfg.CfgDir = "./data"
	TGCfg.SaveDir = "./"
	TGCfg.HttpAddr = ":2020"
	TGCfg.LogLev = 0
	TGCfg.LogSize = 300 << 20
	TGCfg.LogCnt = 1
	TGCfg.cfgPath = filepath.Join(TGCfg.CfgDir, "config.json")
	TGCfg.sessionPath = filepath.Join(TGCfg.CfgDir, "session.json")

	cfg := utils.XmArgValString("cfg", "", "./config.json")
	utils.XmUsageIfHasKeys("h", "help")

	file, err := os.Open(cfg)
	if err != nil {
		logs.Error(err).Str("cfg", cfg).Msg("open fail")
		return
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&TGCfg); err != nil {
		logs.Panic(err).Str("cfg", cfg).Msg("Failed to decode cfg")
	}

	TGCfg.cfgPath = filepath.Join(TGCfg.CfgDir, "config.json")
	TGCfg.sessionPath = filepath.Join(TGCfg.CfgDir, "session.json")
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
