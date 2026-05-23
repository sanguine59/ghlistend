package auth

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const (
	service  = "ghlistend"
	username = "github-pat"
)

var ErrNoToken = errors.New("no token found — run `ghlistend login` first")

func GetToken() (string, error) {
	tok, err := keyring.Get(service, username)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNoToken
	}
	if err != nil {
		return "", err
	}
	return tok, nil
}

func SetToken(token string) error {
	return keyring.Set(service, username, token)
}

func DeleteToken() error {
	err := keyring.Delete(service, username)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNoToken
	}
	return err
}
