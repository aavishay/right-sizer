# Secret Management Guide for Right-Sizer Helm Chart

## Overview

This guide explains how to securely manage sensitive credentials (API tokens, keys, cluster IDs) when deploying the Right-Sizer operator using Helm.

**⚠️ SECURITY WARNING**: Never commit actual secrets, tokens, or API keys to version control!

## Quick Start

### Method 1: Create Secrets During Installation (Recommended for Testing)

```bash
# Install with auto-generated API token
helm install right-sizer ./helm \
  --set dashboard.apiToken.create=true \
  --set dashboard.apiToken.generateRandom=true \
  --set dashboard.url="https://dashboard.example.com" \
  --set dashboard.cluster.secretCreate=true \
  --set dashboard.cluster.name="production-cluster"

# Install with provided API token (use --set-string for complex tokens)
helm install right-sizer ./helm \
  --set dashboard.apiToken.create=true \
  --set-string dashboard.apiToken.value="${DASHBOARD_API_TOKEN}" \
  --set dashboard.url="https://dashboard.example.com" \
  --set dashboard.cluster.secretCreate=true \
  --set dashboard.cluster.name="production-cluster"
```

### Method 2: Pre-create Kubernetes Secrets (Recommended for Production)

```bash
# Create dashboard API token secret
kubectl create secret generic right-sizer-dashboard-token \
  --from-literal=api-token="${DASHBOARD_API_TOKEN}" \
  -n right-sizer

# Create cluster identification secret
kubectl create secret generic right-sizer-cluster-info \
  --from-literal=cluster-id="${CLUSTER_ID}" \
  --from-literal=cluster-name="${CLUSTER_NAME}" \
  -n right-sizer

# Install using existing secrets
helm install right-sizer ./helm \
  --set dashboard.apiToken.existingSecret="right-sizer-dashboard-token" \
  --set dashboard.cluster.existingSecret="right-sizer-cluster-info" \
  --set dashboard.url="https://dashboard.example.com"
```

### Method 3: Using External Secrets Operator (Best for Production)

If you're using [External Secrets Operator](https://external-secrets.io/):

```yaml
# external-secret.yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: right-sizer-dashboard
  namespace: right-sizer
spec:
  secretStoreRef:
    name: vault-backend
    kind: SecretStore
  target:
    name: right-sizer-dashboard
  data:
    - secretKey: api-token
      remoteRef:
        key: right-sizer/dashboard
        property: api-token
```

Then reference in Helm:

```bash
helm install right-sizer ./helm \
  --set dashboard.apiToken.existingSecret="right-sizer-dashboard" \
  --set dashboard.url="https://dashboard.example.com"
```

## Configuration Options

### Dashboard API Token

| Parameter | Description | Default |
|-----------|-------------|---------|
| `dashboard.apiToken.create` | Create a new Secret for the API token | `false` |
| `dashboard.apiToken.value` | Token value (stored in Secret) | `""` |
| `dashboard.apiToken.generateRandom` | Generate random 32-char token | `false` |
| `dashboard.apiToken.existingSecret` | Name of existing Secret to use | `""` |
| `dashboard.apiToken.key` | Key in the Secret containing token | `api-token` |

### Cluster Credentials

| Parameter | Description | Default |
|-----------|-------------|---------|
| `dashboard.cluster.secretCreate` | Create Secret for cluster credentials | `false` |
| `dashboard.cluster.existingSecret` | Use existing Secret | `""` |
| `dashboard.cluster.id` | Unique cluster identifier | `""` |
| `dashboard.cluster.name` | Human-readable cluster name | `""` |
| `dashboard.cluster.idKey` | Key in Secret for cluster ID | `cluster-id` |
| `dashboard.cluster.nameKey` | Key in Secret for cluster name | `cluster-name` |

### AI/LLM API Key (if using AI features)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `aiops.narrative.llm.apiKey.create` | Create Secret for API key | `false` |
| `aiops.narrative.llm.apiKey.value` | API key value | `""` |
| `aiops.narrative.llm.apiKey.existingSecret` | Use existing Secret | `""` |
| `aiops.narrative.llm.apiKey.key` | Key in Secret | `api-key` |

## Security Best Practices

### 1. Never Commit Secrets

**DON'T** do this in values.yaml:
```yaml
# BAD - Never commit actual tokens!
dashboard:
  apiToken:
    value: "sk-1234567890abcdef"  # NEVER DO THIS
```

**DO** use environment variables:
```bash
export DASHBOARD_API_TOKEN=$(vault kv get -field=token secret/right-sizer)
helm install right-sizer ./helm \
  --set-string dashboard.apiToken.value="${DASHBOARD_API_TOKEN}"
```

### 2. Use Separate values Files

Create a `values-secret.yaml` (add to .gitignore):
```yaml
# values-secret.yaml - DO NOT COMMIT
dashboard:
  apiToken:
    create: true
    value: "your-actual-token-here"
```

Install with:
```bash
helm install right-sizer ./helm \
  -f values.yaml \
  -f values-secret.yaml
```

### 3. Rotate Tokens Regularly

```bash
# Generate new token
NEW_TOKEN=$(openssl rand -hex 32)

# Update secret
kubectl create secret generic right-sizer-dashboard-token \
  --from-literal=api-token="${NEW_TOKEN}" \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart deployment to pick up new token
kubectl rollout restart deployment/right-sizer -n right-sizer
```

### 4. Use RBAC to Protect Secrets

```yaml
# secret-reader-role.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: right-sizer-secret-reader
  namespace: right-sizer
rules:
- apiGroups: [""]
  resources: ["secrets"]
  resourceNames: ["right-sizer-dashboard", "right-sizer-cluster"]
  verbs: ["get"]
```

### 5. Enable Encryption at Rest

Ensure your Kubernetes cluster has encryption at rest enabled for etcd:
```bash
# Check if encryption is configured
kubectl get secrets -A -o json | jq '.items[].data' | grep -c "enc:aescbc"
```

## Validation

### Check Secret Creation

```bash
# List secrets in namespace
kubectl get secrets -n right-sizer

# Verify secret contents (base64 encoded)
kubectl get secret right-sizer-dashboard -n right-sizer -o jsonpath='{.data.api-token}' | base64 -d

# Check if deployment is using the secret
kubectl describe deployment right-sizer -n right-sizer | grep -A5 "DASHBOARD_API_TOKEN"
```

### Verify Token is Being Used

```bash
# Check environment variables in running pod
kubectl exec -n right-sizer deployment/right-sizer -- env | grep DASHBOARD

# Check logs for dashboard connection
kubectl logs -n right-sizer deployment/right-sizer | grep -i dashboard
```

## Troubleshooting

### Token Not Being Picked Up

1. Check secret exists:
```bash
kubectl get secret -n right-sizer
```

2. Verify secret has correct data:
```bash
kubectl get secret right-sizer-dashboard -n right-sizer -o yaml
```

3. Check deployment environment variables:
```bash
kubectl get deployment right-sizer -n right-sizer -o yaml | grep -A10 "env:"
```

4. Force pod recreation:
```bash
kubectl rollout restart deployment/right-sizer -n right-sizer
```

### Permission Denied Errors

Ensure the service account has permission to read secrets:
```bash
kubectl auth can-i get secrets --as=system:serviceaccount:right-sizer:right-sizer -n right-sizer
```

### Migration from Plain Text

If upgrading from a version using plain text tokens:

1. Create new secrets with existing values
2. Update Helm values to use `existingSecret`
3. Upgrade the release
4. Verify functionality
5. Remove plain text values from values.yaml

## Examples

### Production Setup with HashiCorp Vault

```bash
# Store in Vault
vault kv put secret/right-sizer \
  api_token="${DASHBOARD_API_TOKEN}" \
  cluster_id="${CLUSTER_ID}" \
  cluster_name="${CLUSTER_NAME}"

# Create External Secret
cat <<EOF | kubectl apply -f -
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: right-sizer-credentials
  namespace: right-sizer
spec:
  secretStoreRef:
    name: vault-backend
    kind: SecretStore
  target:
    name: right-sizer-all-secrets
  data:
    - secretKey: api-token
      remoteRef:
        key: secret/right-sizer
        property: api_token
    - secretKey: cluster-id
      remoteRef:
        key: secret/right-sizer
        property: cluster_id
    - secretKey: cluster-name
      remoteRef:
        key: secret/right-sizer
        property: cluster_name
EOF

# Deploy with external secret
helm install right-sizer ./helm \
  --set dashboard.apiToken.existingSecret="right-sizer-all-secrets" \
  --set dashboard.cluster.existingSecret="right-sizer-all-secrets"
```

### Development Setup

```bash
# Quick setup for development
helm install right-sizer ./helm \
  --create-namespace \
  --namespace right-sizer-dev \
  --set dashboard.apiToken.create=true \
  --set dashboard.apiToken.generateRandom=true \
  --set dashboard.cluster.secretCreate=true \
  --set dashboard.cluster.name="dev-cluster" \
  --set dashboard.url="http://localhost:8080"
```

## Related Documentation

- [Kubernetes Secrets Best Practices](https://kubernetes.io/docs/concepts/security/secrets-good-practices/)
- [External Secrets Operator](https://external-secrets.io/)
- [Sealed Secrets](https://sealed-secrets.netlify.app/)
- [SOPS](https://github.com/mozilla/sops)
