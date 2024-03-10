FROM golang:1.22.0

RUN mkdir /storage_service
COPY go.mod /storage_service
COPY go.sum /storage_service
COPY main.go /storage_service
WORKDIR /storage_service

ENTRYPOINT [ "go", "run", "main.go" ]