# Casty gateway.server
Casty real-time communication server!

## Prerequisites
* First, ensure that you have installed Go 1.11 or higher
* gRPC.server [Casty gRPC.server](https://github.com/castyapp/grpc.server#readme)

## Pull Docker Image
```bash
$ docker pull castyapp/gateway:latest
```

## Run docker container
```bash
$ docker run -p 55283:55283 castyapp/gateway
```

## Docker-Compose example
```yaml
version: '3'

services:
  gateway:
    image: castyapp/gateway:latest
    ports:
      - 3001:3001
      - 3002:3002
    args: ['--config-file', '/config/config.hcl']
    volumes:
      - $PWD/config.hcl:/config/config.hcl
```

## Clone the project
```bash
$ git clone https://github.com/castyapp/gateway.server.git
```

## Configuraition
Make a copy of `example.config.hcl` for your own configuration. save it as `config.hcl` file.
```bash
$ cp example.config.hcl config.hcl
```

### Configure grpc client
You can find more information about how to run grpc server 
is available here [https://github.com/castyapp/grpc.server#readme]
```hcl
grpc {
  host = "localhost"
  port = 55283
}
```

### Redis configuration
Put your redis connection here
```hcl
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
```

You're ready to Go!

## Run project with go compiler
you can simply run the project with following command
```bash
$ go run server.go
```

or if you're considering building the project
```bash
$ go build -o server .
```

## or Build/Run the docker image
```bash
$ docker build . --tag=casty.gateway

$ docker run -d --restart=always -p 3001:3001 -p 3002:3002 casty.gateway
```

## Contributing
Thank you for considering contributing to this project!

## License
Casty is an open-source software licensed under the MIT license.
