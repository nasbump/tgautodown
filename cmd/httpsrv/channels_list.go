package httpsrv

import (
	"encoding/json"
	"net/http"
	"tgautodown/internal/logs"

	"github.com/oklog/ulid/v2"
)

// ChannelsListResponse 频道列表响应结构
type ChannelsListResponse struct {
	Rtn      int      `json:"rtn"`
	Msg      string   `json:"msg"`
	Channels []string `json:"channels"`
}

// HandleChannelsList 处理频道列表查询
func HandleChannelsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rid := ulid.Make().String()
	logs.Info().Rid(rid).Str("path", r.URL.Path).Msg("Channels list request")

	// 从配置中获取频道列表
	response := ChannelsListResponse{
		Rtn: 0,
		Msg: "succ",
		// Channels: logic.TGCfg.ChannelNames,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logs.Warn(err).Rid(rid).Msg("Failed to encode channels list response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
