package deliveryhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	authn "github.com/pashathecreator/holdem/services/wallet-service/internal/auth"
	"github.com/pashathecreator/holdem/services/wallet-service/internal/domain"
)

type authValidator interface {
	AuthenticateRequest(r *http.Request) (*authn.Claims, error)
}

type walletService interface {
	Balance(ctx context.Context, userID string) (*domain.Balance, error)
	DepositAddress(ctx context.Context, userID string) (*domain.DepositAddress, error)
	LinkedAddresses(ctx context.Context, userID string) ([]*domain.LinkedAddress, error)
	CreateLinkChallenge(ctx context.Context, userID, address string) (*domain.LinkChallenge, error)
	ConfirmLink(ctx context.Context, userID, challengeID, address, signature string) (*domain.LinkedAddress, error)
	Deposits(ctx context.Context, userID string) ([]*domain.Deposit, error)
	RequestWithdrawal(ctx context.Context, userID, address string, amountGwei int64) (*domain.WithdrawalRequest, error)
	Withdrawals(ctx context.Context, userID string) ([]*domain.WithdrawalRequest, error)
	CreateAdminFaucetTransfer(ctx context.Context, adminUserID, userID string, amountGwei int64) (*domain.FaucetTransfer, error)
	FaucetTransfers(ctx context.Context) ([]*domain.FaucetTransfer, error)
	DebitForJoin(ctx context.Context, userID, tableID string, amountGwei int64, idempotencyKey string) error
	CreditForCashout(ctx context.Context, userID, tableID string, amountGwei int64, idempotencyKey string) error
}

type Server struct {
	service       walletService
	auth          authValidator
	internalToken string
}

type createLinkChallengeRequest struct {
	Address string `json:"address"`
}

type confirmLinkRequest struct {
	ChallengeID string `json:"challenge_id"`
	Address     string `json:"address"`
	Signature   string `json:"signature"`
}

type createWithdrawalRequest struct {
	Address    string `json:"address"`
	AmountGwei int64  `json:"amount_gwei"`
}

type adminFaucetRequest struct {
	UserID     string `json:"user_id"`
	AmountGwei int64  `json:"amount_gwei"`
}

type internalMoneyRequest struct {
	UserID         string `json:"user_id"`
	TableID        string `json:"table_id"`
	AmountGwei     int64  `json:"amount_gwei"`
	IdempotencyKey string `json:"idempotency_key"`
}

func NewServer(service walletService, auth authValidator, internalToken string) *Server {
	return &Server{service: service, auth: auth, internalToken: internalToken}
}

func (s *Server) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) Balance(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authenticatedUserID(w, r)
	if !ok {
		return
	}
	balance, err := s.service.Balance(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":        balance.UserID,
		"available_gwei": balance.AvailableGwei,
		"updated_at":     balance.UpdatedAt.Format(time.RFC3339),
	})
}

func (s *Server) DepositAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authenticatedUserID(w, r)
	if !ok {
		return
	}
	address, err := s.service.DepositAddress(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":    address.UserID,
		"address":    address.Address,
		"chain":      address.Chain,
		"created_at": address.CreatedAt.Format(time.RFC3339),
	})
}

func (s *Server) LinkedAddresses(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authenticatedUserID(w, r)
	if !ok {
		return
	}
	addresses, err := s.service.LinkedAddresses(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]map[string]string, 0, len(addresses))
	for _, item := range addresses {
		items = append(items, map[string]string{
			"id":          item.ID,
			"address":     item.Address,
			"created_at":  item.CreatedAt.Format(time.RFC3339),
			"verified_at": item.VerifiedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"addresses": items})
}

func (s *Server) CreateLinkChallenge(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authenticatedUserID(w, r)
	if !ok {
		return
	}
	var req createLinkChallengeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	challenge, err := s.service.CreateLinkChallenge(r.Context(), userID, req.Address)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"challenge_id": challenge.ID,
		"address":      challenge.Address,
		"challenge":    challenge.Challenge,
		"expires_at":   challenge.ExpiresAt.Format(time.RFC3339),
	})
}

func (s *Server) ConfirmLink(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authenticatedUserID(w, r)
	if !ok {
		return
	}
	var req confirmLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	linked, err := s.service.ConfirmLink(r.Context(), userID, req.ChallengeID, req.Address, req.Signature)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"id":          linked.ID,
		"address":     linked.Address,
		"verified_at": linked.VerifiedAt.Format(time.RFC3339),
	})
}

func (s *Server) Deposits(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authenticatedUserID(w, r)
	if !ok {
		return
	}
	deposits, err := s.service.Deposits(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deposits": deposits})
}

func (s *Server) CreateWithdrawal(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authenticatedUserID(w, r)
	if !ok {
		return
	}
	var req createWithdrawalRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	withdrawal, err := s.service.RequestWithdrawal(r.Context(), userID, req.Address, req.AmountGwei)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, withdrawal)
}

func (s *Server) Withdrawals(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authenticatedUserID(w, r)
	if !ok {
		return
	}
	withdrawals, err := s.service.Withdrawals(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"withdrawals": withdrawals})
}

func (s *Server) AdminFaucet(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.authenticatedAdmin(w, r)
	if !ok {
		return
	}
	var req adminFaucetRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	transfer, err := s.service.CreateAdminFaucetTransfer(r.Context(), admin.UserID, req.UserID, req.AmountGwei)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, transfer)
}

func (s *Server) ListAdminFaucetTransfers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticatedAdmin(w, r); !ok {
		return
	}
	transfers, err := s.service.FaucetTransfers(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"transfers": transfers})
}

func (s *Server) DebitForJoin(w http.ResponseWriter, r *http.Request) {
	if !s.internalAuthorized(w, r) {
		return
	}
	var req internalMoneyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.service.DebitForJoin(r.Context(), req.UserID, req.TableID, req.AmountGwei, req.IdempotencyKey); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) CreditForCashout(w http.ResponseWriter, r *http.Request) {
	if !s.internalAuthorized(w, r) {
		return
	}
	var req internalMoneyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.service.CreditForCashout(r.Context(), req.UserID, req.TableID, req.AmountGwei, req.IdempotencyKey); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authenticatedUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	user, err := s.auth.AuthenticateRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return "", false
	}
	return user.UserID, true
}

func (s *Server) authenticatedAdmin(w http.ResponseWriter, r *http.Request) (*authn.Claims, bool) {
	user, err := s.auth.AuthenticateRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return nil, false
	}
	if !user.IsAdmin {
		writeError(w, http.StatusForbidden, errors.New("admin access required"))
		return nil, false
	}
	return user, true
}

func (s *Server) internalAuthorized(w http.ResponseWriter, r *http.Request) bool {
	if s.internalToken == "" || strings.TrimSpace(r.Header.Get("X-Internal-Token")) != s.internalToken {
		writeError(w, http.StatusUnauthorized, errors.New("invalid internal token"))
		return false
	}
	return true
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err)
	case errors.Is(err, domain.ErrInsufficientFunds), errors.Is(err, domain.ErrAddressAlreadyLinked), errors.Is(err, domain.ErrAddressNotLinked):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, domain.ErrChallengeNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, domain.ErrChallengeExpired), errors.Is(err, domain.ErrInvalidChallengeProof):
		writeError(w, http.StatusUnauthorized, err)
	case errors.Is(err, domain.ErrAccountNotFound):
		writeError(w, http.StatusNotFound, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

func writeError(w http.ResponseWriter, statusCode int, err error) {
	writeJSON(w, statusCode, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, statusCode int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}

func decodeJSON(r *http.Request, value interface{}) error {
	return json.NewDecoder(r.Body).Decode(value)
}
