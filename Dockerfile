FROM golang:1.13

# Update and install curl
RUN apt-get update

# Creating work directory
RUN mkdir /code

# Adding project to work directory
ADD . /code

# Choosing work directory
WORKDIR /code

# build project
RUN go build -o movie.night.gRPC.server .

EXPOSE 3000

CMD ["./movie.night.gRPC.server"]