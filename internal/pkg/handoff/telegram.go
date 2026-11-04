package handoff

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/ctxscope"
)

const (
	notificationTimeout          = 15 * time.Second
	maximumTelegramResponseBytes = 65536
)

type Telegram struct {
	Endpoint string
	Token    string
	ChatID   int64
	Client   *http.Client
}

func NewTelegram(endpoint, token string, chatID int64) *Telegram {
	return &Telegram{
		Endpoint: endpoint,
		Token:    token,
		ChatID:   chatID,
		Client: &http.Client{
			Timeout: notificationTimeout,
			CheckRedirect: func(
				_ *http.Request, _ []*http.Request,
			) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (t *Telegram) Send(ctx context.Context, message string) error {
	data, err := json.Marshal(struct {
		ChatID         int64  `json:"chat_id"`
		Text           string `json:"text"`
		DisablePreview bool   `json:"disable_web_page_preview"`
	}{ChatID: t.ChatID, Text: message, DisablePreview: true})
	if err != nil {
		return ctxerrors.Wrap(err, "encode Telegram notification")
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		t.Endpoint+"/bot"+t.Token+"/sendMessage",
		bytes.NewReader(data),
	)
	if err != nil {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"invalid Telegram endpoint",
		)
	}

	req.Header.Set("Content-Type", "application/json")

	response, err := t.Client.Do(req)
	if err != nil {
		// Transport errors contain the bot token. Do not propagate their URL.
		return ctxerrors.Wrap(
			commerr.ErrUnavailable,
			"Telegram transport failed",
		)
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			ctxscope.GetLogger(ctx).Warn("close Telegram response failed")
		}
	}()

	var result struct {
		OK bool `json:"ok"`
	}

	decoder := json.NewDecoder(
		io.LimitReader(response.Body, maximumTelegramResponseBytes),
	)
	if err := decoder.Decode(&result); err != nil {
		return ctxerrors.Wrap(
			commerr.ErrUnavailable,
			"invalid Telegram response",
		)
	}

	if response.StatusCode != http.StatusOK || !result.OK {
		return ctxerrors.Wrap(
			commerr.ErrUnavailable,
			"Telegram rejected notification",
		)
	}

	return nil
}
