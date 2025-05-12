FROM golang:1.23-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/remitlane ./cmd/remitlane

FROM alpine:3.21
RUN adduser -D -H remitlane
COPY --from=build /out/remitlane /usr/local/bin/remitlane
USER remitlane
EXPOSE 5121
CMD ["remitlane"]
