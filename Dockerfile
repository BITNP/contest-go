FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/contest-server ./cmd/server

FROM alpine:3.22
RUN adduser -D -u 10001 contest
WORKDIR /app
COPY --from=build /out/contest-server /app/contest-server
# 题库不在镜像里，运行时通过 volume 挂载到 /app/data/problems.yaml
RUN mkdir -p /app/data && chown -R contest:contest /app
USER contest
EXPOSE 8080
ENV ADDR=:8080
ENV BANK_PATH=/app/data/problems.yaml
ENTRYPOINT ["/app/contest-server"]
