#!/bin/zsh

kubectl apply -f webapp-namespace.yaml
kubectl apply -f letsencrypt-staging.yaml
kubectl apply -f webapp-deployment.yaml
kubectl apply -f webapp-service.yaml
kubectl apply -f webapp-ingress.yaml
