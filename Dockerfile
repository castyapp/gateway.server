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

EXPOSE 3001
EXPOSE 3002

ENTRYPOINT ["./casty.gateway.server"]
CMD ["--ug-port", "3001", "--tg-port", "3002"]