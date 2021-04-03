# Debug mode
debug = false

# Application environment
env = "dev"

# Configure grpc client
#
# You can find more information about how to run grpc server 
# is available here [https://github.com/castyapp/grpc.server#readme]
grpc {
  host = "localhost"
  port = 55283
}

# Redis configurations
redis {
  # if you wish to use redis cluster, set this value to true
  # If cluster is true, sentinels is required
  # If cluster is false, addr is required
  cluster     = false
  master_name = "casty"
  addr        = "127.0.0.1:26379"
  sentinels   = [
    "127.0.0.1:26379"
  ]
  pass = "super-secure-password"
  sentinel_pass = "super-secure-sentinels-password"
}

# Sentry config
sentry {
  enabled = false
  dsn     = "sentry.dsn.here"
}
