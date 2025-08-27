#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later


set -e

echo "=== Deleting right-sizer Helm release (if exists) ==="
helm uninstall right-sizer || echo "Helm release not found or already deleted."

echo "=== Deleting right-sizer deployment (if exists) ==="
kubectl delete deployment right-sizer --ignore-not-found

echo "=== Deleting right-sizer RBAC resources ==="
kubectl delete clusterrole right-sizer --ignore-not-found
kubectl delete clusterrolebinding right-sizer --ignore-not-found
kubectl delete serviceaccount right-sizer --namespace default --ignore-not-found

echo "=== Cleaning up right-sizer pods ==="
kubectl delete pod -l app=right-sizer --ignore-not-found

echo "=== Cleanup complete ==="
