# 使用官方的golang镜像作为基础镜像
FROM golang:latest AS build

# 设置工作目录
WORKDIR /go/src/app

# 将本地代码复制到容器中
COPY . .

# 构建Go应用程序
RUN go build -o invoce

# 新建一个小镜像，减少容器大小
FROM alpine:latest

# 设置工作目录
WORKDIR /root/

# 从构建镜像中复制二进制文件
COPY --from=build /go/src/app/invoce .

# 暴露端口
EXPOSE 8889

# 运行应用程序
CMD ["./invoce"]
