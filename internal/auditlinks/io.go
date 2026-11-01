package auditlinks

import (
	"context"

	"sea-noise/internal/vault"
)

func realReadFile(ctx context.Context, name string) (string, error) {
	b, err := vault.ReadFileRetryContext(ctx, name)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
