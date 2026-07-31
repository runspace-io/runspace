#!/usr/bin/env sh
set -eu

for check in 'web:http://localhost:3000' 'gateway:http://localhost:8080/healthz' 'metrics:http://localhost:8080/metrics' 'nats:http://localhost:8222/healthz'; do
  name=${check%%:*}; url=${check#*:}
  curl --fail --silent --show-error --max-time 5 "$url" >/dev/null
  printf 'PASS %s\n' "$name"
done

echo 'Checking NATS publish/consume...'
docker compose --profile tools run --rm nats-box sh -ec '
  nats sub chat.message --count=1 > /tmp/chat-event.out &
  subscriber=$!
  sleep 1
  nats pub chat.message "{\"type\":\"chat.message\",\"text\":\"smoke\"}"
  wait $subscriber
  grep -q smoke /tmp/chat-event.out
'
printf '%s\n' 'PASS nats chat.message publish/consume'
printf '%s\n' 'All smoke checks passed.'
