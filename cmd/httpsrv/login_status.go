package httpsrv

import (
	"encoding/json"
	"net/http"
	"tgautodown/cmd/tg"
	"tgautodown/internal/logs"
	"tgautodown/logic"

	"github.com/oklog/ulid/v2"
)

// LoginStatusResponse 登录状态响应结构
type LoginStatusResponse struct {
	Rtn       int    `json:"rtn"`
	Msg       string `json:"msg"`
	Status    int    `json:"status"`
	AppID     int    `json:"appid"`
	AppHash   string `json:"apphash"`
	Phone     string `json:"phone"`
	FirstName string `json:"firstname"`
	Username  string `json:"username"`
}

// HandleLoginStatus 处理登录状态查询
func HandleLoginStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rid := ulid.Make().String()
	logs.Info().Rid(rid).Str("path", r.URL.Path).Msg("Login status request")

	// 这里应该从实际的登录状态管理中获取状态信息
	// 目前先返回默认的未登录状态
	resp := LoginStatusResponse{
		Rtn:     0,
		Msg:     "succ",
		Status:  tg.TgstatusInit,
		AppID:   logic.TGCfg.AppID,
		AppHash: logic.TGCfg.AppHash,
		Phone:   logic.TGCfg.Phone,
	}
	if logic.Tgs != nil {
		resp.Status = logic.Tgs.Status()
		resp.FirstName = logic.Tgs.FirstName
		resp.Username = logic.Tgs.UserName
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logs.Warn(err).Rid(rid).Msg("Failed to encode login status response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
