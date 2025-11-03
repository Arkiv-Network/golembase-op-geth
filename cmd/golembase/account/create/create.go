package create

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/adrg/xdg"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/golembase/account/pkg/useraccount"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

func Create() *cli.Command {
	return &cli.Command{
		Name:  "create",
		Usage: "Create a new account",
		Action: func(c *cli.Context) error {
			// This creates the directories if they don't already exist
			walletPath, err := xdg.ConfigFile(useraccount.WalletPath)
			if err != nil {
				return fmt.Errorf("failed to create config file path: %w", err)
			}

			fmt.Println("walletPath", walletPath)

			info, err := os.Stat(walletPath)
			// We only care if the error is insufficient permissions. If the file does not
			// exist or its size is zero, we can ignore the error since we are creating it.
			if err == nil {
				if info.Size() != 0 {
					return fmt.Errorf("A wallet already exists at %s", walletPath)
				}
			} else if os.IsPermission(err) {
				return fmt.Errorf("failed to stat walletPath %s: %w", walletPath, err)
			}

			password, err := GetPasswordFromEnvStdinOrPrompt()
			if err != nil {
				return fmt.Errorf("failed to create password: %w", err)
			}

			ks := keystore.NewKeyStore(filepath.Dir(walletPath), keystore.StandardScryptN, keystore.StandardScryptP)
			account, err := ks.NewAccount(password)
			if err != nil {
				return fmt.Errorf("failed to create new account: %w", err)
			}

			created := account.URL.Path
			if created != walletPath {
				if err := os.Rename(created, walletPath); err != nil {
					return fmt.Errorf("failed to rename wallet file: %w", err)
				}
			}

			fmt.Println("New wallet created", walletPath)
			fmt.Println("Address:", account.Address.Hex())

			return nil
		},
	}
}

// GetPasswordFromEnvStdinOrPrompt first checks if the password is set in the environment variable WALLET_PASSWORD, then reads a password from stdin if piped, or interactively if in a terminal
// confirming that the passwords match
func GetPasswordFromEnvStdinOrPrompt() (string, error) {
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

		fmt.Print("Confirm password: ")
		byteConfirm, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return "", err
		}
		confirm := strings.TrimSpace(string(byteConfirm))

		if password != confirm {
			return "", fmt.Errorf("passwords did not match")
		}

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
