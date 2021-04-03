FROM golang:1.15-alpine AS builder

LABEL maintainer="Alireza Josheghani <josheghani.dev@gmail.com>"

# Creating work directory
WORKDIR /build

# Adding project to work directory
ADD . /build

# build project
RUN go build -o server .

FROM alpine

COPY --from=builder /build/server /usr/bin/server

EXPOSE 3001
EXPOSE 3002

ENTRYPOINT ["/usr/bin/server"]
CMD ["--ug-port", "3001", "--tg-port", "3002"]
