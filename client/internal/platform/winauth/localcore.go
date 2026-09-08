package winauth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// VerifyCore checks the locally recorded digest. It establishes file integrity,
// not publisher identity, and never downloads a core or queries a catalog.
func VerifyCore(ctx context.Context, source io.Reader, target Target) error {
	if !target.valid() {
		return failure("authorization_core_unverified")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	reader := contextReader{ctx, source}
	var header [2]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return failure("authorization_core_unverified")
	}
	if string(header[:]) != "MZ" {
		return failure("authorization_core_unverified")
	}
	hash := sha256.New()
	if _, err := io.CopyN(hash, io.MultiReader(bytes.NewReader(header[:]), reader), target.Size); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return failure("authorization_core_unverified")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != target.SHA256 {
		return failure("authorization_core_changed")
	}
	return nil
}

type contextReader struct {
	ctx   context.Context
	input io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.input.Read(p)
}
