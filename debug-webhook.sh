#!/bin/bash

echo "🔍 Smart Scheduler Webhook Debug Script"
echo "========================================"

echo ""
echo "1. Checking Smart Scheduler Pod Status..."
kubectl get pods -n kube-smartscheduler -o wide

echo ""
echo "2. Checking Smart Scheduler Service..."
kubectl get svc -n kube-smartscheduler

echo ""
echo "3. Checking Webhook Configuration..."
kubectl get mutatingwebhookconfiguration | grep smart-scheduler || echo "❌ No Smart Scheduler webhook configuration found"

echo ""
echo "4. Checking if webhook configuration exists with any name..."
kubectl get mutatingwebhookconfiguration -o name | grep -i smart || echo "❌ No webhook config found with 'smart' in name"

echo ""
echo "5. Checking all webhook configurations..."
kubectl get mutatingwebhookconfiguration -o name

echo ""
echo "6. Checking Helm release status..."
helm list -n kube-smartscheduler

echo ""
echo "7. Checking webhook service endpoints..."
kubectl get endpoints -n kube-smartscheduler

echo ""
echo "8. Checking for certificate secrets..."
kubectl get secrets -n kube-smartscheduler | grep cert

echo ""
echo "9. Checking Smart Scheduler logs for webhook server startup..."
kubectl logs -n kube-smartscheduler deployment/kube-smartscheduler-smart-scheduler --tail=20 | grep -i webhook || echo "❌ No webhook-related logs found"

echo ""
echo "10. Checking Smart Scheduler logs for server startup..."
kubectl logs -n kube-smartscheduler deployment/kube-smartscheduler-smart-scheduler --tail=50 | grep -E "(webhook|server|listen)" || echo "❌ No server startup logs found"

echo ""
echo "Debug complete! Run this script on your cluster to diagnose the webhook issue." 