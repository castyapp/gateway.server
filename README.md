# Casty gateway.server
We use this project for real-time communication between our clients and server!

## Prerequisites
* First, ensure that you have installed Go 1.11 or higher since we need the support for Go modules via go mod. [Go modules via `go mod`](https://github.com/golang/go/wiki/Modules)

* gRPC.server **We use gRPC server as rest api!**  [gRPC.server project](https://github.com/CastyLab/grpc.server#readme)

## Clone the project
```bash
$ git clone https://github.com/CastyLab/gateway.server.git
```

## Configuraition
Make a copy of `.env.example` for your own configuration. save it as `.env` file.
```bash
$ cp .env.example .env
```

### gRPC configuration
Put your running grpc.server connection details
```env
GRPC_HOST=localhost
GRPC_PORT=55283
```

You're ready to Go!

## Run project with go compiler
you can simply run the project with following command
* this command with install dependencies and after that will run the project
* this project uses go mod file, You can run this project out of the $GOPAH file!
```bash
$ go run server.go
```

or if you're considering building the project use
```bash
$ go build -o server .
```

## or Build/Run the docker image
```bash
$ docker build . --tag=casty.gateway

$ docker run -dp --restart=always 3000:3000 casty.gateway
```

## Contributing
Thank you for considering contributing to this project!


