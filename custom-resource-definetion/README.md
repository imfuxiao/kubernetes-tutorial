## CRD 示例

### 代码生成器

* 安装`code-generator`

```sh
git clone https://github.com/kubernetes/kubernetes.git
cd kubernetes
./hack/make-rules/build.sh ./vendor/k8s.io/code-generator/cmd/{defaulter-gen,conversion-gen,client-gen,lister-gen,informer-gen,deepcopy-gen,openapi-gen}

mv _output/bin/ $GOPATH/bin/
```


* 项目中添加对codegen项目依赖

`hack/tools.go` 添加以下内容

```go
// +build tools

package hack

import _ "k8s.io/code-generator"
```

`hack/boilerplate.go.txt` 添加license

`hack/update-codegen.sh` 添加代码生成shell, 这里是把`k8s.io/code-generator/update-codegen.sh`中的脚本做了改造

* 使用Makefile生成代码