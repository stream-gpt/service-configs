FROM golang:1.25-alpine

RUN apk add --no-cache git ca-certificates wget
RUN go install github.com/air-verse/air@latest

ENV GOPRIVATE="github.com/Gen-Do/*"

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

EXPOSE 8080

CMD ["air"]

