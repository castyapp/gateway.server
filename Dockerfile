FROM golang:1.14

LABEL maintainer="Alireza Josheghani <josheghani.dev@gmail.com>"

ARG DEBIAN_FRONTEND=noninteractive

# Update and install curl
RUN apt-get update

RUN mkdir /code

COPY . /code

# Choosing work directory
WORKDIR /code

# build project
RUN go build -o casty.gateway.server .

CMD ["./casty.gateway.server"]