package tg

import (
	"context"
	"errors"
	"tgautodown/internal/logs"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// 自定义 Authenticator

func (a TgSuber) Phone(_ context.Context) (string, error) {
	return a.UserPhone, nil
}

func (a TgSuber) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	f := a.getLoginCode
	if f == nil {
		logs.Warn(ErrNoLoginCodeHnd).Send()
		return "", ErrNoLoginCodeHnd
	}

	for {
		if code := f(); code != "" {
			return code, nil
		}
	}
}

func (a TgSuber) Password(_ context.Context) (string, error) {
	logs.Info().Str("F2APassword", a.F2APassword).Msg("enable F2A")
	if a.F2APassword == "" {
		return "", ErrNoF2APassword
	}
	return a.F2APassword, nil
}

func (a TgSuber) AcceptTermsOfService(ctx context.Context, tos tg.HelpTermsOfService) error {
	logs.Info().Msg("auto accept terms of service")
	return nil
}

func (a TgSuber) SignUp(ctx context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("not implemented")
}
