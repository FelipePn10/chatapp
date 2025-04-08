#!/usr/bin/env bash

host="$1"
port="$2"
shift 2
cmd="$@"

until nc -z "$host" "$port"; do
  echo "Aguardando $host:$port..."
  sleep 2
done

exec $cmd