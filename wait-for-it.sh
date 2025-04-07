#!/usr/bin/env bash
# wait-for-it.sh

host="$1"
shift
cmd="$@"

until nc -z ${host}; do
  echo "Waiting for $host..."
  sleep 2
done

exec $cmd
