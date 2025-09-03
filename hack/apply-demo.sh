#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Script to apply demo workload and RightSizerPolicy for testing

set -e

echo "Applying demo workload & policy..."

# Create demo namespace
kubectl create namespace rs-demo --dry-run=client -o yaml | kubectl apply -f -

# Apply demo resources
kubectl apply -n rs-demo -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo-nginx
  labels:
    app: demo-nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: demo-nginx
  template:
    metadata:
      labels:
        app: demo-nginx
    spec:
      containers:
        - name: nginx
          image: nginx:1.27-alpine
          resources:
            requests:
              cpu: 500m
              memory: 512Mi
            limits:
              cpu: 1000m
              memory: 1Gi
          ports:
            - containerPort: 80
---
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: demo-policy
spec:
  enabled: true
  priority: 10
  targetRef:
    kind: Deployment
    namespaces:
      - rs-demo
    labelSelector:
      matchLabels:
        app: demo-nginx
  mode: balanced
  resourceStrategy:
    cpu:
      requestMultiplier: 0.6
      limitMultiplier: 1.0
      targetUtilization: 70
    memory:
      requestMultiplier: 0.5
      limitMultiplier: 1.0
      targetUtilization: 70
  constraints:
    maxChangePercentage: 50
    cooldownPeriod: 2m
EOF

echo "Demo workload & policy applied successfully"
