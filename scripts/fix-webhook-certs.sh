#!/bin/bash

# Script to manually fix webhook certificates
set -e

NAMESPACE="kube-smartscheduler"
SERVICE_NAME="kube-smartscheduler-smart-scheduler-webhook-service"
SECRET_NAME="kube-smartscheduler-smart-scheduler-webhook-server-cert"

echo "🔧 Fixing webhook certificates..."

# Create temporary directory
TMP_DIR=$(mktemp -d)
cd $TMP_DIR

echo "📝 Generating new certificates..."

# Generate CA private key
openssl genrsa -out ca.key 2048

# Generate CA certificate
openssl req -new -x509 -key ca.key -out ca.crt -days 365 -subj "/CN=smart-scheduler-ca"

# Generate server private key  
openssl genrsa -out tls.key 2048

# Generate server certificate signing request
openssl req -new -key tls.key -out server.csr -subj "/CN=${SERVICE_NAME}.${NAMESPACE}.svc"

# Create extensions file for SAN
cat > server.ext <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${SERVICE_NAME}
DNS.2 = ${SERVICE_NAME}.${NAMESPACE}
DNS.3 = ${SERVICE_NAME}.${NAMESPACE}.svc
DNS.4 = ${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local
EOF

# Generate server certificate
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out tls.crt -days 365 -extensions v3_req -extfile server.ext

echo "🔐 Creating Kubernetes secret..."

# Delete existing secret if it exists
kubectl delete secret $SECRET_NAME -n $NAMESPACE --ignore-not-found=true

# Create new secret
kubectl create secret tls $SECRET_NAME \
  --cert=tls.crt \
  --key=tls.key \
  -n $NAMESPACE

# Add CA cert to secret
kubectl patch secret $SECRET_NAME -n $NAMESPACE --type='json' -p='[{"op": "add", "path": "/data/ca.crt", "value": "'$(base64 -w 0 ca.crt)'"}]'

echo "🔄 Updating webhook configuration with new CA bundle..."

# Update webhook configuration with new CA bundle
CA_BUNDLE=$(base64 -w 0 ca.crt)
kubectl patch mutatingwebhookconfiguration kube-smartscheduler-smart-scheduler-mutating-webhook-configuration --type='json' -p='[{"op": "replace", "path": "/webhooks/0/clientConfig/caBundle", "value": "'$CA_BUNDLE'"}]'

echo "♻️  Restarting Smart Scheduler pod..."

# Restart the pod to pick up new certificates
kubectl delete pod -n $NAMESPACE -l app.kubernetes.io/name=smart-scheduler

echo "✅ Certificate fix complete!"

# Cleanup
cd /
rm -rf $TMP_DIR

echo "🔍 Waiting for pod to be ready..."
kubectl wait --for=condition=ready pod -n $NAMESPACE -l app.kubernetes.io/name=smart-scheduler --timeout=60s

echo "🎉 Smart Scheduler is ready with new certificates!" 