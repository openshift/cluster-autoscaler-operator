#!/bin/sh

kustomize build | sudo kubectl apply -f -
