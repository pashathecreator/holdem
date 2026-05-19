package chain

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	gweiWei     = int64(1_000_000_000)
	transferGas = uint64(21_000)
)

type Client struct {
	enabled       bool
	client        *ethclient.Client
	chainID       *big.Int
	hotWalletKey  *ecdsa.PrivateKey
	hotWalletAddr common.Address
}

type ObservedTransfer struct {
	UserID      string
	ToAddress   string
	FromAddress string
	TxHash      string
	ValueGwei   int64
	RawValueWei string
	BlockNumber int64
}

func NewClient(rpcURL, hotWalletAddress, hotWalletPrivateKey string, enabled bool) (*Client, error) {
	if !enabled {
		return &Client{}, nil
	}
	if strings.TrimSpace(rpcURL) == "" || strings.TrimSpace(hotWalletPrivateKey) == "" {
		return nil, fmt.Errorf("chain is enabled but rpc_url or hot_wallet_private_key is empty")
	}

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial ethereum rpc: %w", err)
	}
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("read chain id: %w", err)
	}

	privateKeyHex := strings.TrimPrefix(strings.TrimSpace(hotWalletPrivateKey), "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("parse hot wallet private key: %w", err)
	}
	derivedAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	if addr := strings.TrimSpace(hotWalletAddress); addr != "" && !strings.EqualFold(addr, derivedAddress.Hex()) {
		client.Close()
		return nil, fmt.Errorf("hot wallet address does not match private key")
	}

	return &Client{
		enabled:       true,
		client:        client,
		chainID:       chainID,
		hotWalletKey:  privateKey,
		hotWalletAddr: derivedAddress,
	}, nil
}

func (c *Client) Enabled() bool {
	return c != nil && c.enabled && c.client != nil
}

func (c *Client) HotWalletAddress() string {
	if c == nil {
		return ""
	}
	return c.hotWalletAddr.Hex()
}

func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	c.client.Close()
	return nil
}

func (c *Client) GenerateDepositAccount() (string, string, error) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return "", "", err
	}
	privateBytes := crypto.FromECDSA(privateKey)
	return hex.EncodeToString(privateBytes), crypto.PubkeyToAddress(privateKey.PublicKey).Hex(), nil
}

func (c *Client) RecoverPersonalSignAddress(message, signature string) (string, error) {
	signature = strings.TrimPrefix(strings.TrimSpace(signature), "0x")
	sig, err := hex.DecodeString(signature)
	if err != nil {
		return "", err
	}
	if len(sig) != 65 {
		return "", fmt.Errorf("invalid signature length")
	}
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	if sig[64] > 1 {
		return "", fmt.Errorf("invalid recovery id")
	}

	hash := accounts.TextHash([]byte(message))
	pubkey, err := crypto.SigToPub(hash, sig)
	if err != nil {
		return "", err
	}
	return crypto.PubkeyToAddress(*pubkey).Hex(), nil
}

func (c *Client) CurrentBlock(ctx context.Context) (uint64, error) {
	if !c.Enabled() {
		return 0, fmt.Errorf("chain disabled")
	}
	return c.client.BlockNumber(ctx)
}

func (c *Client) ScanDeposits(ctx context.Context, fromBlock, toBlock uint64, addresses map[string]string) ([]ObservedTransfer, error) {
	if !c.Enabled() {
		return nil, nil
	}
	if fromBlock > toBlock {
		return nil, nil
	}

	observed := make([]ObservedTransfer, 0)
	for blockNumber := fromBlock; blockNumber <= toBlock; blockNumber++ {
		block, err := c.client.BlockByNumber(ctx, new(big.Int).SetUint64(blockNumber))
		if err != nil {
			return nil, fmt.Errorf("load block %d: %w", blockNumber, err)
		}
		signer := types.LatestSignerForChainID(c.chainID)
		for _, tx := range block.Transactions() {
			to := tx.To()
			if to == nil {
				continue
			}
			userID, ok := addresses[strings.ToLower(to.Hex())]
			if !ok {
				continue
			}
			receipt, err := c.client.TransactionReceipt(ctx, tx.Hash())
			if err != nil {
				if errors.Is(err, ethereum.NotFound) {
					continue
				}
				return nil, fmt.Errorf("load receipt for tx %s: %w", tx.Hash().Hex(), err)
			}
			if receipt.Status != types.ReceiptStatusSuccessful {
				continue
			}
			from, err := types.Sender(signer, tx)
			if err != nil {
				return nil, fmt.Errorf("recover sender for tx %s: %w", tx.Hash().Hex(), err)
			}
			valueGwei, err := weiToGwei(tx.Value())
			if err != nil {
				return nil, fmt.Errorf("convert tx value to gwei: %w", err)
			}
			observed = append(observed, ObservedTransfer{
				UserID:      userID,
				ToAddress:   to.Hex(),
				FromAddress: from.Hex(),
				TxHash:      tx.Hash().Hex(),
				ValueGwei:   valueGwei,
				RawValueWei: tx.Value().String(),
				BlockNumber: int64(block.NumberU64()),
			})
		}
	}
	return observed, nil
}

func (c *Client) SubmitSweep(ctx context.Context, fromPrivateKeyHex, toAddress string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("chain disabled")
	}
	privateKeyHex := strings.TrimPrefix(strings.TrimSpace(fromPrivateKeyHex), "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("parse sweep private key: %w", err)
	}
	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	to := common.HexToAddress(toAddress)
	return c.sendFullBalance(ctx, privateKey, fromAddress, to)
}

func (c *Client) SubmitWithdrawal(ctx context.Context, toAddress string, amountGwei int64) (string, int64, error) {
	if !c.Enabled() {
		return "", 0, fmt.Errorf("chain disabled")
	}
	to := common.HexToAddress(toAddress)
	txHash, nonce, err := c.sendValue(ctx, c.hotWalletKey, c.hotWalletAddr, to, gweiToWei(amountGwei))
	if err != nil {
		return "", 0, err
	}
	if nonce > math.MaxInt64 {
		return "", 0, fmt.Errorf("nonce overflow")
	}
	return txHash, int64(nonce), nil
}

func (c *Client) TransactionReceipt(ctx context.Context, txHash string) (*types.Receipt, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("chain disabled")
	}
	receipt, err := c.client.TransactionReceipt(ctx, common.HexToHash(txHash))
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			return nil, nil
		}
		return nil, err
	}
	return receipt, nil
}

func (c *Client) sendFullBalance(ctx context.Context, privateKey *ecdsa.PrivateKey, from, to common.Address) (string, error) {
	balance, err := c.client.BalanceAt(ctx, from, nil)
	if err != nil {
		return "", fmt.Errorf("load sweep balance: %w", err)
	}
	gasPrice, err := c.client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("suggest gas price: %w", err)
	}
	fee := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(transferGas))
	if balance.Cmp(fee) <= 0 {
		return "", fmt.Errorf("insufficient balance to sweep after gas")
	}
	value := new(big.Int).Sub(balance, fee)
	txHash, _, err := c.sendValue(ctx, privateKey, from, to, value)
	return txHash, err
}

func (c *Client) sendValue(ctx context.Context, privateKey *ecdsa.PrivateKey, from, to common.Address, value *big.Int) (string, uint64, error) {
	nonce, err := c.client.PendingNonceAt(ctx, from)
	if err != nil {
		return "", 0, fmt.Errorf("load nonce: %w", err)
	}
	gasPrice, err := c.client.SuggestGasPrice(ctx)
	if err != nil {
		return "", 0, fmt.Errorf("suggest gas price: %w", err)
	}

	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &to,
		Value:    value,
		Gas:      transferGas,
		GasPrice: gasPrice,
	})
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(c.chainID), privateKey)
	if err != nil {
		return "", 0, fmt.Errorf("sign tx: %w", err)
	}
	if err := c.client.SendTransaction(ctx, signed); err != nil {
		return "", 0, fmt.Errorf("send tx: %w", err)
	}
	return signed.Hash().Hex(), nonce, nil
}

func weiToGwei(value *big.Int) (int64, error) {
	gwei := new(big.Int).Div(new(big.Int).Set(value), big.NewInt(gweiWei))
	if !gwei.IsInt64() {
		return 0, fmt.Errorf("value overflows int64 gwei")
	}
	return gwei.Int64(), nil
}

func gweiToWei(value int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(value), big.NewInt(gweiWei))
}
