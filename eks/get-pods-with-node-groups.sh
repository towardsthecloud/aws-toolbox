#!/bin/bash

# Get a list of pods along with the name of the node group
# that pod is running in

set -eu
set -o pipefail

# dump a list of pods and nodes
kubectl get pods --no-headers -o custom-columns=NODE:.spec.nodeName,POD:.metadata.name | sort > /tmp/pods

# dump a list of nodes and node groups (AWS EKS style)
kubectl get nodes -L eks.amazonaws.com/nodegroup | awk '{print $1, $NF}' | sort > /tmp/nodes-and-groups

# join the two files, and print pod and node group
join /tmp/nodes-and-groups /tmp/pods | awk '{print $3, $2}'
