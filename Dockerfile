FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /feed-service ./cmd/feed-service

FROM alpine:3.20
COPY --from=build /feed-service /feed-service
EXPOSE 8080
ENTRYPOINT ["/feed-service"]
