package useraccount

import (
	"bufio"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/adrg/xdg"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/term"
)

type UserAccount struct {
	Address    common.Address
	PrivateKey *ecdsa.PrivateKey
}

func Load() (*UserAccount, error) {
	walletPath, err := xdg.ConfigFile(WalletPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get config file path: %w", err)
	}

	walletBytes, err := os.ReadFile(walletPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read wallet file: %w", err)
	}

	password, err := readPassword()
	if err != nil {
		return nil, fmt.Errorf("failed to read password: %w", err)
	}

	key, err := keystore.DecryptKey(walletBytes, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt private key: %w", err)
	}

	return &UserAccount{
		Address:    crypto.PubkeyToAddress(key.PrivateKey.PublicKey),
		PrivateKey: key.PrivateKey,
	}, nil

}

// readPassword reads a password from stdin if piped, or interactively if in a terminal
func readPassword() (string, error) {
	password, ok := os.LookupEnv("WALLET_PASSWORD")
	if ok {
		return password, nil
	}

	// Check if input is coming from a terminal
	if term.IsTerminal(int(syscall.Stdin)) {

		fmt.Print("Enter wallet password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return "", err
		}
		password := strings.TrimSpace(string(bytePassword))

		return password, nil
	}

	// Otherwise, read from stdin (e.g., piped input)
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(password), nil
}
