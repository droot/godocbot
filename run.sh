#!/bin/bash

GOBIN=${PWD}/bin go install ${PWD#$GOPATH/src/}/cmd/controller-manager
${PWD}/bin/controller-manager -kubeconfig ${HOME}/.kube/config
