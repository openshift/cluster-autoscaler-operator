FROM alpine:3.6

RUN adduser -D cluster-autoscaler-operator
USER cluster-autoscaler-operator

ADD tmp/_output/bin/cluster-autoscaler-operator /usr/local/bin/cluster-autoscaler-operator
