# Reproducible builder image
FROM openshift/origin-release:golang-1.10 as builder

# Workaround a bug in imagebuilder (some versions) where this dir will not be auto-created.
RUN mkdir -p /go/src/github.com/openshift/cluster-autoscaler-operator
WORKDIR /go/src/github.com/openshift/cluster-autoscaler-operator

# This expects that the context passed to the docker build command is
# the cluster-autoscaler-operator directory.
# e.g. docker build -t <tag> -f <this_Dockerfile> <path_to_cluster-autoscaler-operator>
COPY . .

ENV NO_DOCKER=1
ENV GOPATH=/go
ENV BUILD_DEST=/go/bin/cluster-autoscaler-operator

RUN unset VERSION && make build

# Final container
FROM openshift/origin-base
RUN yum install -y ca-certificates

COPY --from=builder /go/bin/cluster-autoscaler-operator /

CMD ["/cluster-autoscaler-operator"]
