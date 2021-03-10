.PHONY: run
run:
	docker run -p 8080:8080 registry.cn-hangzhou.aliyuncs.com/morse_k8s/helloworld:0.1.0
.PHONY: build
build:
	GOOS=linux go build -a --ldflags '-extldflags "-static"' -tags netgo -installsuffix netgo -o helloworld .
.PHONY: image
image: build
	docker build -t registry.cn-hangzhou.aliyuncs.com/morse_k8s/helloworld:0.1.0 .
	docker push registry.cn-hangzhou.aliyuncs.com/morse_k8s/helloworld:0.1.0
	rm helloworld
.PHONY: clean
clean:
	rm hellworld