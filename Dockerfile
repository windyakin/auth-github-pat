FROM golang:1.26 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /auth-github-pat .

FROM gcr.io/distroless/static-debian12

COPY --from=builder /auth-github-pat /auth-github-pat

ENTRYPOINT ["/auth-github-pat"]
