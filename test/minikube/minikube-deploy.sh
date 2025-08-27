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

echo "=== Applying RBAC manifests ==="
kubectl apply -f rbac.yaml

echo "=== Applying deployment manifest ==="
kubectl apply -f deployment.yaml

echo "=== Checking deployment status ==="
kubectl get deployments
kubectl get pods

echo "=== Describing right-sizer pod ==="
kubectl describe pod -l app=right-sizer

echo "=== Operator logs ==="
kubectl logs -l app=right-sizer
