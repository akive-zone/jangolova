#!/bin/sh
set -eu

message="${JANGOLOVA_FIXTURE_MESSAGE:-native process ready}"
printf '%s\n' "${message}" >&2

stopping=0
trap 'stopping=1' INT TERM
while [ "${stopping}" -eq 0 ]; do
  sleep 1
done

printf '%s\n' "native process stopped" >&2
