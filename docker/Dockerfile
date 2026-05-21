# ========== 构建阶段 ==========
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

# 先复制 go.mod/go.sum，利用 Docker 缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 构建
RUN CGO_ENABLED=1 GOOS=linux go build -buildvcs=false -o /app/agw ./cmd/agw/

# ========== 前端构建阶段 ==========
FROM node:20-alpine AS frontend

WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm install

# 复制版本文件到构建上下文
COPY docs/VERSION ../docs/VERSION

COPY web/ .
RUN npm run build

# ========== 运行阶段 ==========
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

# 创建专用插件运行用户（无 sudo、无 shell、无 home、不可登录）
RUN addgroup -g 1001 agw-plugin && \
    adduser -D -H -s /sbin/nologin -G agw-plugin -u 1001 agw-plugin

WORKDIR /app

# 从构建阶段复制二进制
COPY --from=builder /app/agw /app/agw

# 复制版本文件
COPY docs/VERSION /app/docs/VERSION

# 从前端构建阶段复制静态文件
COPY --from=frontend /app/web/dist /app/web/dist

# 创建数据目录 + 权限加固
RUN mkdir -p /app/data /app/data/tmp /app/plugins /app/logs /app/logs/plugins && \
    chmod 700 /app/data && \
    chmod 1777 /app/data/tmp

# 环境变量
ENV AGW_DB_PATH=/app/data/agw.db \
    AGW_SERVER_PORT=7860 \
    AGW_SERVER_MODE=release

EXPOSE 7860

VOLUME ["/app/data", "/app/plugins", "/app/logs"]

ENTRYPOINT ["/app/agw"]
