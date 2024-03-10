FROM golang:1.22.0

RUN mkdir /api_service
COPY . /api_service
WORKDIR /api_service

ENTRYPOINT [ "go", "run", "main.go" ]