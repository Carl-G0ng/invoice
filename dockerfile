# 使用官方的golang镜像作为基础镜像
FROM golang:1.22.2 AS build

MAINTAINER gyh

VOLUME /home/invoice

RUN mkdir -p /home/invoice

# 设置工作目录
WORKDIR /home/invoice

# 将本地代码复制到容器中
COPY ./ /home/invoice/

# 构建Go应用程序
RUN go build -o invoice \
    go env -w GO111MODULE=on  \
    go env -w GOPROXY="https://goproxy.io,direct" \
#    go env -w GOPRIVATE="*.corp.example.com"  \
#    go env -w GOPRIVATE="*.corp.example.com" \
#    go env -w GOPRIVATE="example.com/org_name" \


# 暴露端口
EXPOSE 9204

# 运行应用程序
CMD ["/home/invoice/invoice"]