#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later


set -e

echo "=== Starting Minikube ==="
minikube start

echo "=== Setting kubectl context to minikube ==="
kubectl config use-context minikube

echo "=== Compiling right-sizer operator ==="
go mod tidy
go build -o right-sizer main.go

echo "=== Switching Docker to Minikube's daemon ==="
eval $(minikube docker-env)

echo "=== Building Docker image ==="
docker build -t right-sizer:latest .

echo "=== Resetting Docker to host daemon ==="
eval $(minikube docker-env -u)

echo "=== Creating Helm chart if not present ==="
CHART_DIR="charts/right-sizer"
if [ ! -d "$CHART_DIR" ]; then
  echo "Helm chart directory $CHART_DIR not found!"
  exit 1
fi

echo "=== Installing/Upgrading right-sizer operator via Helm ==="
helm upgrade --install right-sizer $CHART_DIR \
  --set image.repository=right-sizer \
  --set image.tag=latest \
  --set image.pullPolicy=IfNotPresent

echo "=== Checking deployment status ==="
kubectl get deployments
kubectl get pods

echo "=== Describing right-sizer pod ==="
kubectl describe pod -l app=right-sizer

echo "=== Operator logs ==="
kubectl logs -l app=right-sizer
