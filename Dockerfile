# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/remitlane ./cmd/remitlane

FROM alpine:3.21
RUN adduser -D -H remitlane
COPY --from=build /out/remitlane /usr/local/bin/remitlane
ENV PORT=5121
EXPOSE 5121
USER remitlane
CMD ["remitlane"]
