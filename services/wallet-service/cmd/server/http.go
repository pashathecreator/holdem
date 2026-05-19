package main

import (
	"net/http"

	deliveryhttp "github.com/pashathecreator/holdem/services/wallet-service/internal/delivery/http"
)

func buildHTTPServer(addr string, server *deliveryhttp.Server) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.Healthz)
	mux.HandleFunc("GET /v1/wallet/balance", server.Balance)
	mux.HandleFunc("GET /v1/wallet/deposit-address", server.DepositAddress)
	mux.HandleFunc("GET /v1/wallet/addresses", server.LinkedAddresses)
	mux.HandleFunc("POST /v1/wallet/addresses/link-challenge", server.CreateLinkChallenge)
	mux.HandleFunc("POST /v1/wallet/addresses/link-confirm", server.ConfirmLink)
	mux.HandleFunc("GET /v1/wallet/deposits", server.Deposits)
	mux.HandleFunc("POST /v1/wallet/withdrawals", server.CreateWithdrawal)
	mux.HandleFunc("GET /v1/wallet/withdrawals", server.Withdrawals)
	mux.HandleFunc("POST /v1/admin/faucet/transfers", server.AdminFaucet)
	mux.HandleFunc("GET /v1/admin/faucet/transfers", server.ListAdminFaucetTransfers)
	mux.HandleFunc("POST /internal/v1/wallet/debit-for-join", server.DebitForJoin)
	mux.HandleFunc("POST /internal/v1/wallet/credit-for-cashout", server.CreditForCashout)
	return &http.Server{Addr: addr, Handler: mux}
}
