package httpsrv

import (
	"encoding/json"
	"net/http"
	"tgautodown/internal/logs"
	"tgautodown/logic"

	"github.com/oklog/ulid/v2"
)

// LoginCodeRequest 登录验证码请求结构
type LoginCodeRequest struct {
	Code string `json:"code"`
}

// LoginCodeResponse 登录验证码响应结构
type LoginCodeResponse struct {
	Rtn int    `json:"rtn"`
	Msg string `json:"msg"`
}

// HandleLoginCode 处理登录验证码提交
func HandleLoginCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rid := ulid.Make().String()
	logs.Info().Rid(rid).Str("path", r.URL.Path).Msg("Login code request")

	var req LoginCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logs.Warn(err).Rid(rid).Msg("Failed to decode login code request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 验证必填字段
	if req.Code == "" {
		logs.Warn(nil).Rid(rid).Msg("Missing required fields in login code request")
		http.Error(w, "Missing code field", http.StatusBadRequest)
		return
	}

	logic.InputLoginCode(req.Code)
	logs.Info().
		Str("code", req.Code).
		Msg("Received login verification code")

	response := LoginCodeResponse{
		Rtn: 0,
		Msg: "succ",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logs.Warn(err).Rid(rid).Err(err).Msg("Failed to encode login code response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
