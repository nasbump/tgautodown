package httpsrv

import (
	"encoding/json"
	"net/http"
	"tgautodown/internal/logs"
	"tgautodown/logic"

	"github.com/oklog/ulid/v2"
)

// LoginUserRequest 用户登录请求结构
type LoginUserRequest struct {
	AppID   int    `json:"appid"`
	AppHash string `json:"apphash"`
	Phone   string `json:"phone"`
}

// LoginUserResponse 用户登录响应结构
type LoginUserResponse struct {
	Rtn int    `json:"rtn"`
	Msg string `json:"msg"`
}

// HandleLoginUser 处理用户登录信息提交
func HandleLoginUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rid := ulid.Make().String()
	logs.Info().Rid(rid).Str("path", r.URL.Path).Msg("Login user request")

	var req LoginUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logs.Warn(err).Rid(rid).Msg("Failed to decode login user request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 验证必填字段
	if req.AppID == 0 || req.AppHash == "" || req.Phone == "" {
		logs.Warn(nil).Rid(rid).Int("appid", req.AppID).Str("apphash", req.AppHash).Str("phone", req.Phone).Msg("arg miss")
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	logic.TGCfg.AppID = req.AppID
	logic.TGCfg.AppHash = req.AppHash
	logic.TGCfg.Phone = req.Phone

	logic.SaveCfg()
	go logic.TgSuberStart()

	response := LoginUserResponse{
		Rtn: 0,
		Msg: "succ",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logs.Warn(err).Rid(rid).Msg("Failed to encode login status response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
