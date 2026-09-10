package main

import (
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/tyler-smith/go-bip39"
)

func DerivePrivateKey(mnemonic string, pathStr string) (*ecdsa.PrivateKey, error) {
	seed := bip39.NewSeed(mnemonic, "") // Add passphrase support here if needed.
	masterKey, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
	if err != nil {
		return nil, err
	}

	// Parse the path, for example, "m/44'/60'/0'/0/0".
	components := strings.Split(strings.TrimPrefix(pathStr, "m/"), "/")
	key := masterKey

	for _, component := range components {
		var index uint32
		isHardened := strings.HasSuffix(component, "'")
		cleanComp := strings.TrimSuffix(component, "'")

		_, err := fmt.Sscanf(cleanComp, "%d", &index)
		if err != nil {
			return nil, fmt.Errorf("invalid path segment: %s", component)
		}

		if isHardened {
			index += hdkeychain.HardenedKeyStart
		}

		key, err = key.Derive(index)
		if err != nil {
			return nil, err
		}
	}

	privKeyBytes, err := key.ECPrivKey()
	if err != nil {
		return nil, err
	}

	return crypto.ToECDSA(privKeyBytes.ToECDSA().D.Bytes())
}
