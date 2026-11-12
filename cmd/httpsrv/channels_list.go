package httpsrv

import (
	"encoding/json"
	"net/http"
	"tgautodown/cmd/tg"
	"tgautodown/internal/logs"
	"tgautodown/logic"

	"github.com/oklog/ulid/v2"
)

// ChannelsListResponse 频道列表响应结构
type ChannelsListResponse struct {
	Rtn      int                `json:"rtn"`
	Msg      string             `json:"msg"`
	Channels []tg.ChannelConfig `json:"channels"`
}

// HandleChannelsList 处理频道列表查询
func HandleChannelsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rid := ulid.Make().String()
	logs.Info().Rid(rid).Str("path", r.URL.Path).Msg("Channels list request")

	channels := logic.GetChannels()
	if channels == nil {
		channels = []tg.ChannelConfig{}
	}
	response := ChannelsListResponse{
		Rtn:      0,
		Msg:      "succ",
		Channels: channels,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logs.Warn(err).Rid(rid).Msg("Failed to encode channels list response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
