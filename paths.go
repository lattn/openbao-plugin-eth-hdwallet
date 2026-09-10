package main

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
	"github.com/tyler-smith/go-bip39"
)

func pathRootKey(b *ethBackend) *framework.Path {
	return &framework.Path{
		Pattern: "root-key",
		Fields: map[string]*framework.FieldSchema{
			"mnemonic": {
				Type:        framework.TypeString,
				Description: "BIP-39 Mnemonic phrase (Optional. Will generate one if empty).",
			},
			"force": {
				Type:        framework.TypeBool,
				Description: "Force overwrite existing root key (Requires high privilege).",
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathRootKeyWrite,
			},
		},
	}
}

func pathAddress(b *ethBackend) *framework.Path {
	return &framework.Path{
		Pattern: "address",
		Fields: map[string]*framework.FieldSchema{
			"path": {
				Type:        framework.TypeString,
				Description: "BIP-44 HD Derivation Path (eg: m/44'/60'/0'/0/0)",
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathAddressRead,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathAddressRead,
			},
		},
	}
}

func pathSignTx(b *ethBackend) *framework.Path {
	return &framework.Path{
		Pattern: "sign-tx",
		Fields: map[string]*framework.FieldSchema{
			"path":                     {Type: framework.TypeString, Description: "HD Derivation Path (e.g. m/44'/60'/0'/0/0)"},
			"chain_id":                 {Type: framework.TypeInt, Description: "Chain ID (e.g. 1 for Mainnet)"},
			"nonce":                    {Type: framework.TypeInt64, Description: "Account Nonce"},
			"to":                       {Type: framework.TypeString, Description: "Recipient Address"},
			"value":                    {Type: framework.TypeString, Description: "Value in Wei"},
			"gas_limit":                {Type: framework.TypeInt64, Description: "Gas Limit"},
			"max_fee_per_gas":          {Type: framework.TypeString, Description: "EIP-1559 Max Fee"},
			"max_priority_fee_per_gas": {Type: framework.TypeString, Description: "EIP-1559 Priority Fee"},
			"data":                     {Type: framework.TypeString, Description: "Transaction Hex Data"},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathSignTxWrite,
			},
		},
	}
}

func (b *ethBackend) pathRootKeyWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	existing, err := req.Storage.Get(ctx, "root-key")
	if err != nil {
		return nil, err
	}

	force := data.Get("force").(bool)
	if existing != nil && !force {
		return logical.ErrorResponse("root key already exists, set force=true to overwrite."), nil
	}

	mnemonic, ok := data.Get("mnemonic").(string)
	if !ok {
		return logical.ErrorResponse("invalid mnemonic type"), nil
	}

	show := mnemonic == ""
	if mnemonic == "" {
		// Generate a 24-word mnemonic when none is provided.
		entropy, _ := bip39.NewEntropy(256)
		mnemonic, _ = bip39.NewMnemonic(entropy)
	}

	if !bip39.IsMnemonicValid(mnemonic) {
		return logical.ErrorResponse("invalid mnemonic"), nil
	}

	entry, err := logical.StorageEntryJSON("root-key", map[string]string{
		"mnemonic": mnemonic,
	})
	if err != nil {
		return nil, err
	}

	// Store the mnemonic in OpenBao's encrypted internal storage.
	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	res := map[string]any{
		"message": "mnemonic successfully saved",
	}
	if show {
		res["mnemonic"] = mnemonic
	}

	return &logical.Response{
		Data: res,
	}, nil
}

func (b *ethBackend) pathAddressRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	// Get the HD derivation path parameter.
	path := data.Get("path").(string)
	if path == "" {
		path = "m/44'/60'/0'/0/0" // Default path for the first Ethereum address.
	}

	// Read the mnemonic from OpenBao's encrypted storage.
	entry, err := req.Storage.Get(ctx, "config")
	if err != nil || entry == nil {
		return logical.ErrorResponse("plugin unconfigured. Please run /config first."), nil
	}

	var config map[string]string
	if err := entry.DecodeJSON(&config); err != nil {
		return nil, err
	}

	// Derive the ECDSA private key in memory.
	privKey, err := DerivePrivateKey(config["mnemonic"], path)
	if err != nil {
		return logical.ErrorResponse(fmt.Sprintf("failed to derive key: %s", err)), nil
	}

	// Generate the standard Ethereum address (0x...) from the public key.
	address := crypto.PubkeyToAddress(privKey.PublicKey).Hex()

	return &logical.Response{
		Data: map[string]any{
			"path":    path,
			"address": address,
		},
	}, nil
}

func (b *ethBackend) pathSignTxWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	// 1. Retrieve the stored mnemonic.
	entry, err := req.Storage.Get(ctx, "config")
	if err != nil || entry == nil {
		return logical.ErrorResponse("plugin not configured with mnemonic"), nil
	}

	var config map[string]string
	if err := entry.DecodeJSON(&config); err != nil {
		return nil, err
	}

	// 2. Parse the request parameters.
	path := data.Get("path").(string)
	chainID := big.NewInt(int64(data.Get("chain_id").(int)))
	toAddr := common.HexToAddress(data.Get("to").(string))

	val := new(big.Int)
	val.SetString(data.Get("value").(string), 10)

	maxFee := new(big.Int)
	maxFee.SetString(data.Get("max_fee_per_gas").(string), 10)

	maxPriority := new(big.Int)
	maxPriority.SetString(data.Get("max_priority_fee_per_gas").(string), 10)

	txData := common.FromHex(data.Get("data").(string))

	// 3. Build the DynamicFeeTx (EIP-1559).
	txInner := &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     uint64(data.Get("nonce").(int64)),
		GasTipCap: maxPriority,
		GasFeeCap: maxFee,
		Gas:       uint64(data.Get("gas_limit").(int64)),
		To:        &toAddr,
		Value:     val,
		Data:      txData,
	}
	rawTx := types.NewTx(txInner)

	// 4. Derive the private key and sign the transaction.
	privKey, err := DerivePrivateKey(config["mnemonic"], path)
	if err != nil {
		return logical.ErrorResponse(fmt.Sprintf("failed to derive key: %s", err)), nil
	}

	signer := types.LatestSignerForChainID(chainID)
	signedTx, err := types.SignTx(rawTx, signer, privKey)
	if err != nil {
		return logical.ErrorResponse(fmt.Sprintf("failed to sign tx: %s", err)), nil
	}

	// 5. Encode as RLP hex for broadcast by the client.
	signedTxBytes, err := signedTx.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]any{
			"raw_tx": fmt.Sprintf("0x%x", signedTxBytes),
			"hash":   signedTx.Hash().Hex(),
		},
	}, nil
}
