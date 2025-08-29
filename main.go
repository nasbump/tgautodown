package main

import (
	"embed"
	"tgautodown/cmd/httpsrv"
	"tgautodown/logic"
)

//go:embed static/*
var embeddedStaticFiles embed.FS

func main() {
	logic.ParseCfg()

	if logic.TGCfg.AppID > 0 && logic.TGCfg.AppHash != "" && logic.TGCfg.Phone != "" {
		go logic.TgSuberStart()
	}

	httpsrv.StartHttpSrv(embeddedStaticFiles, logic.TGCfg.HttpAddr)
}
