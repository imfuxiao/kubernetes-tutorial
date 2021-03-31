## 监控部署

### TLS自签名证书

```sh
# Generate the CA cert and private key
> openssl req -nodes -new -x509 -days 3650 -keyout ca.key -out ca.pem -subj "/CN=fuxiao"
# Generate the private key for the webhook server
> openssl genrsa -out key.pem 2048
# Generate a Certificate Signing Request (CSR) for the private key, and sign it with the private key of the CA.
> openssl req -new -key key.pem -subj "/CN=*.fuxiao.dev" \
| openssl x509 -days 3650 -req -CA ca.pem -CAkey ca.key -CAcreateserial -out cert.pem
```

* CN: Common Name，浏览器使用该字段验证网站是否合法，一般写的是域名。非常重要。浏览器使用该字段验证网站是否合法

```sh
openssl x509 -in ca.pem -text -noout
openssl x509 -in cert.pem -text -noout
```

### Prometheus/Grafana/AlerterManager 部署

