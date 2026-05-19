package walletclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pashathecreator/holdem/services/table-manager/internal/domain"
)

type Client struct {
	baseURL       string
	internalToken string
	httpClient    *http.Client
}

type moneyRequest struct {
	UserID         string `json:"user_id"`
	TableID        string `json:"table_id"`
	AmountGwei     int64  `json:"amount_gwei"`
	IdempotencyKey string `json:"idempotency_key"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func New(baseURL, internalToken string) *Client {
	return &Client{
		baseURL:       strings.TrimRight(baseURL, "/"),
		internalToken: internalToken,
		httpClient:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) DebitForJoin(ctx context.Context, userID, tableID string, amountGwei int64, idempotencyKey string) error {
	return c.postMoney(ctx, "/internal/v1/wallet/debit-for-join", moneyRequest{
		UserID:         userID,
		TableID:        tableID,
		AmountGwei:     amountGwei,
		IdempotencyKey: idempotencyKey,
	})
}

func (c *Client) CreditForCashout(ctx context.Context, userID, tableID string, amountGwei int64, idempotencyKey string) error {
	return c.postMoney(ctx, "/internal/v1/wallet/credit-for-cashout", moneyRequest{
		UserID:         userID,
		TableID:        tableID,
		AmountGwei:     amountGwei,
		IdempotencyKey: idempotencyKey,
	})
}

func (c *Client) postMoney(ctx context.Context, path string, payload moneyRequest) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal wallet request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build wallet request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", c.internalToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("wallet request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	var response errorResponse
	_ = json.NewDecoder(resp.Body).Decode(&response)
	if resp.StatusCode == http.StatusConflict && strings.Contains(strings.ToLower(response.Error), "insufficient") {
		return domain.ErrInsufficientFunds
	}
	if response.Error == "" {
		response.Error = resp.Status
	}
	return fmt.Errorf("wallet request failed: %s", response.Error)
}
