FROM golang:1.22.0

RUN mkdir /stat_service
COPY . /stat_service
WORKDIR /stat_service

ENTRYPOINT [ "go", "run", "main.go" ]