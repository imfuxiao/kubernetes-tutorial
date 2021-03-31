ROOT_PACKAGE=github.com/imfuxiao/kubernetes-tutorial/custom-resource-definetion/k8s-controller-crd-resource
# API Group
CUSTOM_RESOURCE_NAME="etcd"
# API Version: 多个版本使用逗号分隔
CUSTOM_RESOURCE_VERSION="v1alpha1"
# output-base 需要回到多少级上级目录? 需要看ROOT_PATH的深度
# 另外, 请项目路径命令与$ROOT_PACKAGE保持一致
.PHONY: generator_code
generator_code:
	./hack/update-codegen.sh all \
		$(ROOT_PACKAGE)/pkg/client \
		$(ROOT_PACKAGE)/pkg/apis \
		"$(CUSTOM_RESOURCE_NAME):$(CUSTOM_RESOURCE_VERSION)" \
		--go-header-file "$(PWD)/hack/boilerplate.go.txt" \
		--output-base "$(PWD)/../../../../../" \
		--v 1

.PHONY: build
build:
	GOOS=linux go build -a --ldflags '-extldflags "-static"' -tags netgo -installsuffix netgo -o etcd-controller .

.PHONY: image
image: build
	docker build -t registry.cn-hangzhou.aliyuncs.com/morse_k8s/etcd-controller:0.1.0 .
	docker push registry.cn-hangzhou.aliyuncs.com/morse_k8s/etcd-controller:0.1.0
	rm etcd-controller
