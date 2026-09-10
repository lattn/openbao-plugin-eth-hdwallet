## How to use

```bash
# config/bao.hcl

plugin_directory = "/openbao/plugins"
```

```bash
GOOS=linux GOARCH=amd64 go build -o plugins/openbao-plugin-eth-hdwallet

sha256sum plugins/openbao-plugin-eth-hdwallet | cut -d' ' -f1

bao plugin register -sha256="${SHA256}" secret openbao-plugin-eth-hdwallet

bao secrets enable -path=eth-hd openbao-plugin-eth-hdwallet

bao write eth-hd/root-key mnemonic="" force=false

bao read eth-hd/address path=""

bao write eth-hd/sign-tx \
    path="m/44'/60'/0'/0/0" \
    chain_id=1 \
    nonce=0 \
    to="0x742d35Cc6634C0532925a3b844Bc454e4438f44e" \
    value="1000000000000000000" \
    gas_limit=21000 \
    max_fee_per_gas="20000000000" \
    max_priority_fee_per_gas="1000000000" \
    data="0x"
```