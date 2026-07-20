FROM golang:1.25-alpine AS production_builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /usr/local/bin/main ./cmd/api
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /usr/local/bin/worker ./cmd/worker

FROM golang:1.25 AS development

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential ca-certificates git \
    && rm -rf /var/lib/apt/lists/*

ENV PATH="/go/bin:${PATH}"

RUN go install github.com/air-verse/air@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY ./swagger ./swagger

RUN mkdir -p ./tmp && chmod -R 754 ./tmp

EXPOSE 8080

CMD ["air", "-c", "air.toml"]

FROM alpine:3.22.2 AS production

WORKDIR /app

RUN addgroup -S nonroot && adduser -S nonroot -G nonroot && apk add --no-cache ca-certificates curl

COPY --from=production_builder /usr/local/bin/main /app/main
COPY --from=production_builder /usr/local/bin/worker /app/worker
COPY --from=production_builder /app/swagger /app/swagger

RUN chmod +x /app/main /app/worker

USER nonroot

EXPOSE 8080

CMD ["/app/main"]
