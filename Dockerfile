# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/remitlane ./cmd/remitlane

# Distroless: no shell, no zoneinfo (the binary embeds time/tzdata).
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/remitlane /usr/local/bin/remitlane
ENV PORT=5121
EXPOSE 5121
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/remitlane"]
CMD ["serve"]
