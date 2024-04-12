#!/usr/bin/env bash

set -ex

if docker ps >/dev/null; then
    echo "selecting docker as container runtime"
    docker run --rm -v "${PWD}":/local openapitools/openapi-generator-cli generate --additional-properties=interfaceOnly=true  -i /local/api/swagger.yaml -g go-gin-server -o /local/api
elif ctr c ls >/dev/null; then
    echo "selecting containerd as container runtime"
    ctr run -t --rm --mount type=bind,src="${PWD}",dst=/local,options=rbind:rw docker.io/openapitools/openapi-generator-cli:latest a docker-entrypoint.sh generate --additional-properties=interfaceOnly=true -i /local/api/swagger.yaml -g go-gin-server -o /local/api
else
    echo "no working container runtime found. Neither docker nor containerd seems to work."
    exit 1
fi

# delete useless file
useless_file=(
"api/.openapi-generator"
"api/.openapi-generator-ignore"
"api/Dockerfile"
"api/main.go"
"api/go.mod"
"api/api"
)

sudo rm -rf "${useless_file[@]}"

