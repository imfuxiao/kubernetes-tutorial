## MutatingAdmissionWebhook tutorial

### 使用`openssl`生成证书

```sh
# Generate the CA cert and private key
> openssl req -nodes -new -x509 -keyout ca.key -out ca.crt -subj "/CN=Admission Controller Webhook Envoy CA"
# Generate the private key for the webhook server
> openssl genrsa -out envoy-init-svc-tls-key.pem 2048
# Generate a Certificate Signing Request (CSR) for the private key, and sign it with the private key of the CA.
> openssl req -new -key envoy-init-svc-tls-key.pem -subj "/CN=envoy-init.envoy.svc" \
    | openssl x509 -req -CA ca.crt -CAkey ca.key -CAcreateserial -out envoy-init-svc-tls-cert.pem
```

### 使用 `cfssl`工具生成证书

#### 工具安装

```sh
# 安装工具
> go get github.com/cloudflare/cfssl/cmd/...
```

#### 生成CA密钥(ca-key.pem)和CA证书(ca.pem)

* CA 证书签名请求（CSR）

```sh
# 生成CA 证书签名请求（CSR）
cfssl print-defaults csr > ca-csr.json
```

CN: Common Name，浏览器使用该字段验证网站是否合法，一般写的是域名。非常重要。浏览器使用该字段验证网站是否合法.

C: Country， 国家.

L: Locality，地区，城市.

O: Organization Name，组织名称，公司名称.

OU: Organization Unit Name，组织单位名称，公司部门.

ST: State，州，省

```json
{
    "CN": "kubernetes",
    "key": {
        "algo": "rsa",
        "size": 2048
    },
    "names": [
        {
            "C": "CHINA",
            "ST": "SHAANXI",
            "L": "XIAN"
        }
    ]
}
```
* CA证书配置文件
```sh
cfssl print-defaults config > ca-config.json
```

```json
{
  "signing": {
    "default": {
      "expiry": "8760h"
    },
    "profiles": {
      "kubernetes": {
        "expiry": "8760h",
        "usages": [
          "signing",
          "key encipherment",
          "server auth",
          "client auth"
        ]
      }
    }
  }
}
```

expiry: 证书过期时间, 8760h = 1年

```sh
cfssl gencert -initca ca-csr.json | cfssljson -bare ca
```

#### 为envoy-init生成证书密钥(envoy-key.pem)和证书(envoy.pem)

生成envoy证书配置

```sh
cfssl print-defaults csr > envoy.json
```

```json
{
  "CN": "envoy-init",
  "hosts": [
    "127.0.0.1",
    "envoy-init",
    "envoy-init.envoy",
    "envoy-init.envoy.svc",
    "envoy-init.envoy.svc.cluster",
    "envoy-init.envoy.svc.cluster.local"
  ],
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
      "C": "China",
      "ST": "SHAANXI",
      "L": "XIAN"
    }
  ]
}
```

> 注意CN名称

生成envoy证书

```sh
cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json -profile=kubernetes envoy.json | cfssljson -bare envoy
```

### 如何使用证书

* envoy.pem, envoy-key.pem是为了给admission客户端启动使用
* ca.pem是给kubernetes的apiserver调用使用