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

	go httpsrv.StartHttpSrv(embeddedStaticFiles, logic.TGCfg.HttpAddr)

	if logic.TGCfg.AppID > 0 && logic.TGCfg.AppHash != "" && logic.TGCfg.Phone != "" {
		logic.TgSuberStart()
	}

	select {}
}
