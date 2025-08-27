package httpsrv

import (
	"embed"
	"io/fs"
	"net/http"
	"tgautodown/internal/logs"
)

func StartHttpSrv(embeddedStaticFiles embed.FS, addr string) error {
	// 使用嵌入的静态文件系统
	staticFS, err := fs.Sub(embeddedStaticFiles, ".")
	if err != nil {
		logs.Fatal(err).Msg("Failed to get embedded static filesystem, using fallback")
		return err
	}

	fsHandler := http.FileServer(http.FS(staticFS))

	// 处理根路径，重定向到index.html
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 如果请求的是根路径，返回index.html
		logs.Debug().Str("req", r.URL.Path).Str("from", r.RemoteAddr).Send()
		if r.URL.Path == "/" {
			// 从嵌入的文件系统中提供index.html
			http.ServeFileFS(w, r, staticFS, "static/index.html")
			return
		}

		// // 设置正确的MIME类型
		// if strings.HasSuffix(r.URL.Path, ".css") {
		// 	w.Header().Set("Content-Type", "text/css")
		// } else if strings.HasSuffix(r.URL.Path, ".js") {
		// 	w.Header().Set("Content-Type", "application/javascript")
		// }

		// 处理静态文件请求
		fsHandler.ServeHTTP(w, r)
	})

	// 单独处理API接口
	http.HandleFunc("/tgad/login/status", HandleLoginStatus)
	http.HandleFunc("/tgad/login/user", HandleLoginUser)
	http.HandleFunc("/tgad/login/code", HandleLoginCode)
	http.HandleFunc("/tgad/channels/list", HandleChannelsList)
	http.HandleFunc("/tgad/channels/modify", HandleChannelsModify)

	logs.Info().Str("addr", addr).Msg("HTTP server running with embedded static files")

	return http.ListenAndServe(addr, nil)
}
