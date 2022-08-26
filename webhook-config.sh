#!/usr/bin/env bash

export ca_string=$(cat ${HOME}/.karmada/ca.crt | base64 | tr "\n" " "|sed s/[[:space:]]//g)
export temp_path=$(mktemp -d)

cp -rf "examples/customresourceinterpreter/webhook-configuration.yaml" "${temp_path}/temp.yaml"
sed -i'' -e "s/{{caBundle}}/${ca_string}/g" "${temp_path}/temp.yaml"
kubectl --kubeconfig $HOME/.kube/karmada.config --context karmada-apiserver apply -f "${temp_path}/temp.yaml"
rm -rf "${temp_path}"