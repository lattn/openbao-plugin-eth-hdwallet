storage "raft" {
  path = "/openbao/data"
  node_id = "raft_node_1"
}

listener "tcp" {
  address     = "0.0.0.0:8200"
  tls_disable = 1
}

plugin_directory = "/openbao/plugins"
ui = true
api_addr = "http://127.0.0.1:8200"
cluster_addr = "http://127.0.0.1:8201"