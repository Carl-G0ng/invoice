.PHONY: build run clean

build:
	docker build -t invoice .

run: build
	docker run -p 9204:9204 invoice
