package main

import (
	"context"

	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

type ethBackend struct {
	*framework.Backend
}

func Factory(ctx context.Context, conf *logical.BackendConfig) (logical.Backend, error) {
	b := &ethBackend{}

	b.Backend = &framework.Backend{
		Help: "OpenBao Ethereum HD Wallet Signing Plugin",
		Paths: []*framework.Path{
			pathRootKey(b), // Configure/manage the mnemonic.
			pathAddress(b), // Derive an Ethereum address.
			pathSignTx(b),  // Sign an Ethereum transaction.
		},
		BackendType: logical.TypeLogical,
	}

	if err := b.Setup(ctx, conf); err != nil {
		return nil, err
	}

	return b, nil
}
