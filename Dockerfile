FROM golang:1.13

LABEL maintainer="Alireza Josheghani <josheghani.dev@gmail.com>"

ARG GITLAB_ACCESS_TOKEN
ARG DEBIAN_FRONTEND=noninteractive

# Update and install curl
RUN apt-get update

RUN git config --global url."https://oauth2:${GITLAB_ACCESS_TOKEN}@gitlab.com/".insteadOf "https://gitlab.com/"

# Creating work directory
RUN mkdir /code

# Adding project to work directory
ADD . /code

# Choosing work directory
WORKDIR /code

# build project
RUN go build -o movie.night.ws.server .

EXPOSE 3000

CMD ["./movie.night.ws.server"]