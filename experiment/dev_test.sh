#!/bin/zsh

export IMG=vcsomor/webapp-operator:dev
make undeploy
make\
&& make docker-build \
&& make deploy \
&& kubectl apply -f experiment/letsencrypt-staging-cluster.yaml
sleep 2
kubectl \
  logs \
    -n webapp-operator-system \
    "$(kubectl get po -n webapp-operator-system --selector=control-plane=controller-manager -o jsonpath='{.items[0].metadata.name}')" \
    -c manager \
    -f
