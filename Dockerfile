FROM golang:1.13

LABEL maintainer="Alireza Josheghani <josheghani.dev@gmail.com>"

ARG GITLAB_ACCESS_TOKEN
ARG DEBIAN_FRONTEND=noninteractive

# Update and install curl
RUN apt-get update

RUN git config --global url."https://oauth2:${GITLAB_ACCESS_TOKEN}@gitlab.com/".insteadOf "https://gitlab.com/"

RUN mkdir /code

COPY . /code

# Choosing work directory
WORKDIR /code

# build project
RUN go build -o gateway.server .

EXPOSE 3000

CMD ["./gateway.server"]