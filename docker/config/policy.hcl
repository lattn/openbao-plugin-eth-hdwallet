path "eth-hd/root-key" {
  capabilities = ["deny"]
}

path "eth-hd/sign-tx" {
  capabilities = ["update"]
}

path "eth-hd/address" {
  capabilities = ["read"]
}