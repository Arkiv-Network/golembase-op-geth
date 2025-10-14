package testutil

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

type GethInstance struct {
	*gethProcess
	shutdown    func()
	ETHClient   *ethclient.Client
	RPCClient   *rpc.Client
	RPCEndpoint string
	WALDir      string
}

type gethProcess struct {
	*exec.Cmd
	Stdout io.ReadCloser
	Stderr io.ReadCloser
	output *bytes.Buffer
}

func startGethInstance(ctx context.Context, gethPath string, tempDir string) (_ *GethInstance, err error) {
	// Start geth in dev mode

	td, err := os.MkdirTemp("", "geth-dev-wal")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	walDir := filepath.Join(td, "geth-dev-wal")

	geth, err := startGethWithPath(
		ctx,
		gethPath,
		"--dev",             // Run in dev mode
		"--dev.period", "0", // Mine blocks immediately
		"--http",           // Enable the HTTP-RPC server
		"--ipcdisable",     // Disable ipc, to avoid concurrency issues (using the same socket path)
		"--http.port", "0", // Use random port
		"--http.api", "eth,web3,net,debug,golembase", // Enable necessary APIs
		"--verbosity", "3", // Increase logging to see HTTP endpoint
		"--golembase.sqlstatefile", filepath.Join(tempDir, "golem-base.db"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start geth: %w", err)
	}

	defer func() {
		if err != nil {
			ek := geth.Process.Kill()
			if ek != nil {
				err = errors.Join(err, ek)
			}
			_, we := geth.Process.Wait()
			if we != nil {
				err = errors.Join(err, we)
			}
			os.RemoveAll(td)
		}
	}()

	// Get the HTTP endpoint from geth's output
	endpoint, err := getHTTPEndpoint(ctx, geth)
	if err != nil {
		return nil, fmt.Errorf("failed to get HTTP endpoint: %w", err)
	}

	// Wait for HTTP endpoint to be ready
	err = waitForEndpoint(ctx, endpoint, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for endpoint: %w", err)
	}

	// Connect to the node
	client, err := ethclient.Dial(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to dial ethclient: %w", err)
	}

	rpcClient, err := rpc.Dial(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to dial rpc client: %w", err)
	}

	// Return cleanup function
	cleanup := func() {
		client.Close()
		rpcClient.Close()
		geth.Process.Kill()
	}

	gi := &GethInstance{
		gethProcess: geth,
		ETHClient:   client,
		RPCClient:   rpcClient,
		RPCEndpoint: endpoint,
		shutdown:    cleanup,
		WALDir:      walDir,
	}

	return gi, nil
}

func startGethWithPath(ctx context.Context, gethPath string, args ...string) (*gethProcess, error) {

	cmd := exec.CommandContext(ctx, gethPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start geth: %w", err)
	}
	return &gethProcess{
		cmd,
		stdout,
		stderr,
		&bytes.Buffer{},
	}, nil
}

func getHTTPEndpoint(ctx context.Context, geth *gethProcess) (string, error) {
	// Regular expression to match the HTTP endpoint line
	portPattern := regexp.MustCompile(`HTTP server started\s+endpoint=[^:]+:(\d+)`)

	// Buffer to store outputBuffer for error reporting
	var outputBuffer bytes.Buffer

	mux := io.MultiWriter(geth.output, &outputBuffer)

	// Create a multiplexer to read from both stdout and stderr
	go io.Copy(mux, geth.Stdout)
	scanner := bufio.NewScanner(io.TeeReader(geth.Stderr, mux))

	// Channel to receive the endpoint
	endpointCh := make(chan string, 1)
	errCh := make(chan error, 1)

	// Start scanning in a goroutine
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			if matches := portPattern.FindStringSubmatch(line); matches != nil {
				port, _ := strconv.Atoi(matches[1])
				endpointCh <- fmt.Sprintf("http://127.0.0.1:%d", port)
				// return
			}
		}
		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("error reading geth output: %v", err)
		} else {
			errCh <- fmt.Errorf("HTTP endpoint not found in output")
		}
	}()

	// Wait for either context cancellation or endpoint found
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("timeout waiting for HTTP endpoint. Output:\n%s", outputBuffer.String())
	case endpoint := <-endpointCh:
		return endpoint, nil
	case err := <-errCh:
		return "", fmt.Errorf("%v\nOutput:\n%s", err, outputBuffer.String())
	}
}

// waitForEndpoint waits for the HTTP endpoint to be ready
func waitForEndpoint(ctx context.Context, endpoint string, timeout time.Duration) error {
	client := http.Client{
		Timeout: timeout,
	}

	tContex, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		err := func() error {

			req, err := http.NewRequestWithContext(tContex, http.MethodGet, endpoint, nil)
			if err != nil {
				return err
			}

			res, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to connect to HTTP endpoint: %w", err)
			}

			defer res.Body.Close()

			if res.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected status code: %d", res.StatusCode)
			}

			return nil
		}()

		if err == nil {
			return nil
		}

		if ctx.Err() != nil {
			return fmt.Errorf("timeout waiting for HTTP endpoint: %w", err)
		}

	}

}

type FundedAccount struct {
	PrivateKey *ecdsa.PrivateKey
	Address    common.Address
}

func (g *GethInstance) createAccountAndTransferFunds(ctx context.Context, amount *big.Int) (_ *FundedAccount, err error) {

	acc := &FundedAccount{}

	// Create a new private key and derive the address
	acc.PrivateKey, err = crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	acc.Address = crypto.PubkeyToAddress(acc.PrivateKey.PublicKey)

	// Get dev account using eth_accounts RPC call
	var accounts []common.Address
	err = g.RPCClient.CallContext(ctx, &accounts, "eth_accounts")
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts from the node: %w", err)
	}

	if len(accounts) == 0 {
		return nil, errors.New("no accounts found on the node")
	}

	devAccount := accounts[0] // dev mode account is the first one

	// Transfer ETH using eth_sendTransaction
	var txHash common.Hash
	err = g.RPCClient.CallContext(ctx, &txHash, "eth_sendTransaction", map[string]interface{}{
		"from":  devAccount,
		"to":    acc.Address,
		"value": (*hexutil.Big)(amount),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	// Wait for transaction to be mined
	var receipt *types.Receipt
	for range 25 {
		receipt, err = g.ETHClient.TransactionReceipt(ctx, txHash)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return nil, errors.New("timeout waiting for transaction receipt")
		case <-time.After(time.Second):
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get transaction receipt: %w", err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil, errors.New("transaction failed")
	}

	return acc, nil
}

func EthToWei(n int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(n), big.NewInt(params.Ether))
}
