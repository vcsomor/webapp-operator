#!/bin/zsh

kubectl delete -f webapp-ingress.yaml
kubectl delete -f webapp-service.yaml
kubectl delete -f webapp-deployment.yaml
kubectl delete -f letsencrypt-staging.yaml
kubectl delete -f webapp-namespace.yaml
