#!/bin/bash

set -e

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
source "$(dirname "${BASH_SOURCE}")/lib/yq.sh"

TRUST_MANAGER_VERSION=${1:?"missing trust-manager version. Please specify a version from https://github.com/cert-manager/trust-manager/releases"}

MANIFESTS_PATH=./_output/trust-manager-manifests

mkdir -p "${MANIFESTS_PATH}"

echo "---- Downloading trust-manager manifests ${TRUST_MANAGER_VERSION} ----"

./bin/helm repo add jetstack https://charts.jetstack.io --force-update
./bin/helm template trust-manager jetstack/trust-manager \
    -n cert-manager --version "${TRUST_MANAGER_VERSION}" > "${MANIFESTS_PATH}/manifests.yaml"

echo "---- Patching manifest ----"

./bin/yq e 'del(.metadata.labels."helm.sh/chart")' -i "${MANIFESTS_PATH}/manifests.yaml"
./bin/yq e 'del(.spec.template.metadata.labels."helm.sh/chart")' -i "${MANIFESTS_PATH}/manifests.yaml"
./bin/yq e 'del(.spec.template.metadata.labels."app.kubernetes.io/managed-by")' -i "${MANIFESTS_PATH}/manifests.yaml"

./bin/yq e \
  '(.. | select(has("app.kubernetes.io/managed-by"))."app.kubernetes.io/managed-by") |= "cert-manager-operator"' \
  -i "${MANIFESTS_PATH}/manifests.yaml"

./bin/yq e 'select(.kind == "CustomResourceDefinition").metadata.labels."app" = "trust-manager"' -i "${MANIFESTS_PATH}/manifests.yaml"

rm -rf bindata/trust-manager
mkdir -p bindata/trust-manager

./bin/yq --output-format json \
    eval-all '.' -I 0 \
    "${MANIFESTS_PATH}/manifests.yaml" | while read -r item; do

  name=$(echo "$item" | ./bin/yq eval '.metadata.name' -)
  kind=$(echo "$item" | ./bin/yq eval '.kind' - | tr '[:upper:]' '[:lower:]')

  if [[ "${kind}" == "customresourcedefinition" ]]; then
    mkdir -p config/crd/bases
    output_file="config/crd/bases/${name}.yaml"
  else
    output_file="bindata/trust-manager/${name}-${kind}.yaml"
  fi

  echo "$item" | ./bin/yq eval -P > "$output_file"
  echo "$output_file"
done

rm -rf "${MANIFESTS_PATH}"
