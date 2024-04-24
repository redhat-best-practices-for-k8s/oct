#!/bin/bash

# This script is used to test the endpoints of the API
# to make sure they are alive and healthy

# Define the endpoints
endpoints=(
  "https://catalog.redhat.com/api/containers/v1/operators/bundles"
  "https://catalog.redhat.com/api/containers/v1/images"
  "https://catalog.redhat.com/api/containers/v1/ping"
  "https://catalog.redhat.com/api/containers/v1/images?filter=certified==true&page_size=100&page=0&include=data.repositories,data.image_id,data.architecture,data.repositories.manifest_list_digest"
  "https://charts.openshift.io/index.yaml"
  "https://catalog.redhat.com/api/containers/v1/operators/bundles?filter=organization==certified-operators&page_size=100&page=0"
  "https://catalog.redhat.com/api/containers/v1/operators/bundles?filter=organization==certified-operators"
)

# Loop through the endpoints and exit if any of them fail
for endpoint in "${endpoints[@]}"
do
  printf "Testing endpoint: $endpoint\n"
  curl -s -o /dev/null -w "%{http_code}" $endpoint
  printf "\n"
  if [ $? -ne 0 ]; then
    echo "Failed to reach endpoint: $endpoint"
    exit 1
  fi
done
