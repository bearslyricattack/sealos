FROM gcr.io/distroless/static:nonroot
ARG TARGETARCH
COPY bin/service-vlogs-$TARGETARCH /manager
# 设置环境变量
ENV WHITELIST_KUBERNETES_HOSTS="https://usw.sailos.io:6443"
EXPOSE 8428
USER 65532:65532

ENTRYPOINT [ "/manager", "config/config.yml" ]