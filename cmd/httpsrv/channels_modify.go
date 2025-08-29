package httpsrv

import (
	"encoding/json"
	"net/http"
	"tgautodown/internal/logs"
	"tgautodown/logic"

	"github.com/oklog/ulid/v2"
)

// ChannelsModifyResponse 频道修改响应结构
type ChannelsModifyResponse struct {
	Rtn int    `json:"rtn"`
	Msg string `json:"msg"`
}

// HandleChannelsModify 处理频道修改
func HandleChannelsModify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rid := ulid.Make().String()
	logs.Info().Rid(rid).Str("path", r.URL.Path).Msg("Channels modify request")

	var channels []string
	if err := json.NewDecoder(r.Body).Decode(&channels); err != nil {
		logs.Warn(err).Rid(rid).Msg("Failed to decode channels modify request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 更新频道列表
	// logic.TGCfg.ChannelNames = channels
	logic.SaveCfg()

	response := ChannelsModifyResponse{
		Rtn: 0,
		Msg: "succ",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logs.Warn(err).Rid(rid).Msg("Failed to encode channels modify response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
