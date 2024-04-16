.PHONY: all build run clean

all: build

build:
	go build -o invoice

run: build
	# 检查端口是否被占用
	if sudo lsof -i :8889 -t &>/dev/null; then \
		echo "端口 8889 已被占用，终止占用该端口的进程"; \
		sudo lsof -i :8889 -t | xargs -r sudo kill -9; \
	fi
	nohup sudo ./invoice &

clean:
	rm -f invoice

kill:
	if sudo lsof -i :8889 -t &>/dev/null; then \
		echo "终止端口 8889"; \
		sudo lsof -i :8889 -t | xargs -r sudo kill -9; \
	fi