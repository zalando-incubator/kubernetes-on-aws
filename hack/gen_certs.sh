#!/bin/bash

credentials_dir=credentials

mkdir -p $credentials_dir

cat > $credentials_dir/apiserver.conf << EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = kubernetes
DNS.2 = kubernetes.default
DNS.3 = kubernetes.default.svc
DNS.4 = kubernetes.default.svc.cluster.local
IP.1 = 172.17.4.101
IP.2 = 10.3.0.1
EOF

cat > $credentials_dir/client.conf << EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
EOF

# Create a certificate authority
openssl genrsa -out $credentials_dir/ca-key.pem 2048
openssl req -x509 -new -nodes -key $credentials_dir/ca-key.pem -days 100000 -out $credentials_dir/ca.pem -subj "/O=k8s-aws-hack/CN=hack-ca"

# Create a server certiticate
openssl genrsa -out $credentials_dir/apiserver-key.pem 2048
openssl req -new -key $credentials_dir/apiserver-key.pem -out $credentials_dir/apiserver.csr -subj "/O=k8s-aws-hack/CN=k8s-aws-hack-apiserver" -config $credentials_dir/apiserver.conf
openssl x509 -req -in $credentials_dir/apiserver.csr -CA $credentials_dir/ca.pem -CAkey $credentials_dir/ca-key.pem -CAcreateserial -out $credentials_dir/apiserver.pem -days 100000 -extensions v3_req -extfile $credentials_dir/apiserver.conf

# Create a client certiticate for worker nodes
openssl genrsa -out $credentials_dir/worker-key.pem 2048
openssl req -new -key $credentials_dir/worker-key.pem -out $credentials_dir/worker.csr -subj "/O=k8s-aws-hack/CN=k8s-aws-hack-worker" -config $credentials_dir/client.conf
openssl x509 -req -in $credentials_dir/worker.csr -CA $credentials_dir/ca.pem -CAkey $credentials_dir/ca-key.pem -CAcreateserial -out $credentials_dir/worker.pem -days 100000 -extensions v3_req -extfile $credentials_dir/client.conf

# Create a client certiticate for admin user
openssl genrsa -out $credentials_dir/admin-key.pem 2048
openssl req -new -key $credentials_dir/admin-key.pem -out $credentials_dir/admin.csr -subj "/O=k8s-aws-hack/CN=k8s-aws-hack-admin" -config $credentials_dir/client.conf
openssl x509 -req -in $credentials_dir/admin.csr -CA $credentials_dir/ca.pem -CAkey $credentials_dir/ca-key.pem -CAcreateserial -out $credentials_dir/admin.pem -days 100000 -extensions v3_req -extfile $credentials_dir/client.conf
