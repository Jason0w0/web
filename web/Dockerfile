FROM ghcr.io/a-h/templ:latest AS template
WORKDIR /app
COPY --chown=65532:65532 home.templ .
RUN ["templ", "generate"]

FROM golang:1.23-alpine AS build
RUN apk add --update gcc musl-dev
WORKDIR /web
COPY main.go handlers.go .env .
COPY ./config /web/config
COPY go.mod go.sum .
COPY --from=template /app /web
RUN go mod tidy
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o webapp .

FROM alpine:latest AS final
WORKDIR /app
COPY ./static /app/static
COPY --from=build /web/webapp .
COPY --from=build /web/.env .
EXPOSE 3000
CMD ["./webapp"]

