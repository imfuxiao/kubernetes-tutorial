.PHONY: run
run:
	docker run -p 8443:443 \
	-v /Users/morse/github/kubernetes-tutorial/dynamic-admission-control/mutation/deploy/tls:/etc/tls \
	-v /Users/morse/.kube/:/.kube \
	registry.cn-hangzhou.aliyuncs.com/morse_k8s/envoy-init:0.1.0 \
		-tls-cert-file=/etc/tls/svc.pem \
		-tls-private-key-file=/etc/tls/svc-key.pem \
		-kube-config-path=/.kube/config

.PHONY: build
build:
	GOOS=linux go build -a --ldflags '-extldflags "-static"' -tags netgo -installsuffix netgo -o envoy-init .

.PHONY: image
image: build
	docker build -t registry.cn-hangzhou.aliyuncs.com/morse_k8s/envoy-init:0.1.0 .
	docker push registry.cn-hangzhou.aliyuncs.com/morse_k8s/envoy-init:0.1.0
	rm envoy-init

.PHONY: clean
clean:
	rm envoy-init