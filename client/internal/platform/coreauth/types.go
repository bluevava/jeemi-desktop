// Package coreauth authorizes one verified Linux mihomo file. It never starts
// mihomo, installs a service, stores passwords, or accepts commands from the UI.
package coreauth

import (
	"errors"

	"jeemi/internal/platform/requirements"
)

const HelperName = "jeemi-authorizer"
const maxBinarySize = 512 << 20

// helperSHA256 is pinned to the separately built, static helper at release time.
var helperSHA256 string

var ErrCancelled = errors.New("core authorization cancelled")

type Target struct {
	ExecutablePath string `json:"executablePath"`
	SHA256         string `json:"sha256"`
	Size           int64  `json:"size"`
}

type response struct {
	Code string `json:"code"`
}

func failure(code string) error {
	return &requirements.Error{Code: code, Message: code}
}
