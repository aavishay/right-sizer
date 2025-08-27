#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later


set -e

echo "=== 1. Starting Minikube ==="
minikube start

echo "=== 2. Setting kubectl context to minikube ==="
kubectl config use-context minikube

echo "=== 3. Compiling right-sizer operator ==="
go mod tidy
go build -o right-sizer main.go

echo "=== 4. Switching Docker to Minikube's daemon ==="
eval $(minikube docker-env)

echo "=== 5. Building Docker image ==="
docker build -t right-sizer:latest .

echo "=== 6. Resetting Docker to host daemon ==="
eval $(minikube docker-env -u)

echo "=== 7. Installing/Upgrading right-sizer operator via Helm ==="
helm upgrade --install right-sizer ./charts/right-sizer \
  --set image.repository=right-sizer \
  --set image.tag=latest \
  --set image.pullPolicy=IfNotPresent

echo "=== 8. Deploying sample nginx workload ==="
kubectl apply -f nginx-deployment.yaml

echo "=== 9. Waiting for pods to be ready ==="
kubectl wait --for=condition=available deployment/right-sizer --timeout=120s
kubectl wait --for=condition=available deployment/nginx --timeout=120s

echo "=== 10. Operator and workload status ==="
kubectl get deployments
kubectl get pods

echo "=== 11. Operator logs ==="
kubectl logs -l app=right-sizer

echo "=== 12. Nginx deployment resources ==="
kubectl get deployment nginx -o yaml | grep -A20 'resources:'

# Optional: Generate load (uncomment if you want to exec into nginx pod)
# echo "=== 13. Generating load on nginx ==="
# POD=$(kubectl get pod -l app=nginx -o jsonpath='{.items[0].metadata.name}')
# kubectl exec -it $POD -- /bin/sh

# Optional: Cleanup (uncomment to clean up after test)
# echo "=== 14. Cleaning up ==="
# ./minikube-cleanup.sh

echo "=== All steps complete! ==="
