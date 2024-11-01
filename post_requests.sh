#!/bin/bash

url="http://localhost:8080/api/submit"

while true; do
  for i in {1..5}; do
    curl -s -X POST -o /dev/null -w "%{http_code}\n" $url &
  done
  sleep 1
done
