# 使用多阶段构建
FROM golang:1.23 AS builder

# 设置工作目录
WORKDIR /app

# 复制项目所有文件
COPY . .

# 设置环境变量
ENV WHITELIST_KUBERNETES_HOSTS="https://usw.sailos.io:6443"

# 下载依赖并构建 Go 可执行文件
RUN go mod tidy && go build -o vlogs-server main.go

# 使用 Distroless 运行时镜像
FROM gcr.io/distroless/static:nonroot

# 复制构建好的可执行文件
COPY --from=builder /app/service-launchpad /manager

# 暴露端口
EXPOSE 8428

# 运行非 root 用户
USER 65532:65532

# 运行应用
ENTRYPOINT [ "/manager", "config/config.yml" ]
