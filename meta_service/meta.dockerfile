FROM golang:1.22.0

RUN mkdir /meta_service
COPY . /meta_service
WORKDIR /meta_service

ENTRYPOINT [ "go", "run", "main.go" ]