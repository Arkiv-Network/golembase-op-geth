package importkey

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/golembase/account/create"
	"github.com/ethereum/go-ethereum/cmd/golembase/account/pkg/useraccount"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/urfave/cli/v2"
)

func ImportAccount() *cli.Command {
	return &cli.Command{
		Name:  "import",
		Usage: "Import an account using a hex private key",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "privatekey",
				Aliases:  []string{"key"},
				Usage:    "Private key in hex format",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			hexKey := c.String("privatekey")
			hexKey = strings.TrimPrefix(hexKey, "0x")
			privateKeyBytes, err := crypto.HexToECDSA(hexKey)
			if err != nil {
				return fmt.Errorf("invalid private key: %w", err)
			}

			walletPath, err := xdg.ConfigFile(useraccount.WalletPath)
			if err != nil {
				return fmt.Errorf("failed to get config file path: %w", err)
			}

			password, err := create.GetPasswordFromEnvStdinOrPrompt()
			if err != nil {
				return fmt.Errorf("failed to create password: %w", err)
			}

			ks := keystore.NewKeyStore(filepath.Dir(walletPath), keystore.StandardScryptN, keystore.StandardScryptP)
			account, err := ks.ImportECDSA(privateKeyBytes, password)
			if err != nil {
				return fmt.Errorf("failed to encrypt keystore: %w", err)
			}

			imported := account.URL.Path
			if imported != walletPath {
				if err := os.Rename(imported, walletPath); err != nil {
					return fmt.Errorf("failed to rename wallet file: %w", err)
				}
			}

			fmt.Println("Successfully imported account")
			fmt.Println("Address:", account.Address.Hex())

			return nil
		},
	}
}
