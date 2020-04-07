#!/bin/sh
REPO_NAME=$(basename "${PWD}")
if [ "$IS_CONTAINER" != "" ]; then
  go vet "${@}"
else
  docker run --rm \
    --env GO111MODULE="$GO111MODULE" \
    --env GOFLAGS="$GOFLAGS" \
    --env GOPROXY="$GOPROXY" \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/go/src/github.com/openshift/${REPO_NAME}:z" \
    --workdir "/go/src/github.com/openshift/${REPO_NAME}" \
    openshift/origin-release:golang-1.13 \
    ./hack/go-vet.sh "${@}"
fi;
