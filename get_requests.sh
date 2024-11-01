#!/bin/bash

url="http://localhost:8080/api/data"

while true; do
  for i in {1..10}; do
    curl -s -o /dev/null -w "%{http_code}\n" $url &
  done
  sleep 1
done
