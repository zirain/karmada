#!/usr/bin/env bash

VERSION=latest make image-karmada-controller-manager

kind load docker-image swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-controller-manager:latest --name karmada-host

kubectl rollout restart deploy/karmada-controller-manager -nkarmada-system \
    --kubeconfig /root/.kube/karmada.config --context karmada-host