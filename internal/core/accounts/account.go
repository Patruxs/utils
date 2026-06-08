package accounts

import (
	"errors"
	"strings"
)

type Account struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	TOTPSecret string `json:"totp_secret"`
}

type ImportResult struct {
	Accounts []Account
	Skipped  int
}

var ErrNoAccounts = errors.New("no valid accounts found")

func ParseBulkImport(input string) ImportResult {
	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	result := ImportResult{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 3 {
			result.Skipped++
			continue
		}

		account := Account{
			Email:      strings.TrimSpace(parts[0]),
			Password:   parts[1],
			TOTPSecret: strings.TrimSpace(parts[2]),
		}
		if account.Email == "" || account.Password == "" || account.TOTPSecret == "" {
			result.Skipped++
			continue
		}
		result.Accounts = append(result.Accounts, account)
	}

	return result
}

func MaskedEmails(accounts []Account) []string {
	emails := make([]string, 0, len(accounts))
	for _, account := range accounts {
		emails = append(emails, account.Email)
	}
	return emails
}
