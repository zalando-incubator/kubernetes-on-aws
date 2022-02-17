#!/usr/bin/env bash

function help {
  echo "$0 --zone <zone> --target <target>"
}

ZONE=""
TARGET=""
verbose=0
sleep_seconds=0
run_timeout=0

if [ -n "${1+x}" ]
then
while :
do
    case $1 in
        -h | --help | -\?)
          help
          exit 0 # This is not an error, User asked help. Don't do "exit 1"
          ;;
        --zone)
          ZONE=$2
          shift 2
          ;;
        --target)
          TARGET=$2
          shift 2
          ;;
        --timeout)
          run_timeout=$2
          shift 2
          ;;
        --wait)
          sleep_seconds=$2
          shift 2
          ;;
        -v | --verbose)
          # Each instance of -v adds 1 to verbosity
          verbose=$((verbose+1))
          shift
          ;;
        --) # End of all options
          shift
          break
          ;;
        -*)
          printf >&2 'WARN: Unknown option (ignored): %s\n' "$1"
          shift
          ;;
        *)  # no more options. Stop while loop
          break
          ;;
    esac
done
fi

set -euo pipefail

# Suppose some options are required. Check that we got them.

if [ ! "$ZONE" ]; then
    printf >&2 "ERROR: option '--zone <zone>' not given. See --help\n"
    exit 1
fi
if [ ! "$TARGET" ]; then
    printf >&2 "ERROR: option '--target <target>' not given. See --help\n"
    exit 1
fi

if [ $verbose -gt 0 ]
then
  pwd
  ls -l
  ls -l ./loadtest
  ls -l ./loadtest/backend
  ls -l ./loadtest/client
  echo "ZONE ${ZONE}"
  echo "TARGET ${TARGET}"
  echo "URL: https://${TARGET}.${ZONE}"
fi

if [ -s "$ZONE" ]
then
  echo "ERROR: ZONE not defined"
fi
if [ "x$ZONE" == "x" ]
then
  echo "ERROR: empty DNS ZONE"
fi

sed -i "s/%ZONE%/$ZONE/" ./loadtest/backend/*.yaml
sed -i "s/%ZONE%/$ZONE/" ./loadtest/client/*.yaml
sed -i "s/%TARGET%/$TARGET/" ./loadtest/backend/*.yaml
sed -i "s/%TARGET%/$TARGET/" ./loadtest/client/*.yaml

echo "provision namespace: loadtest-e2e"
kubectl create namespace loadtest-e2e

echo "provision loadtest backend with target https://${TARGET}.${ZONE}"
kubectl create -f ./loadtest/backend

# we do not want to fail on failing curl
set +e
while [ "$run_timeout" -gt 0 ]
do
  run_timeout=$(( run_timeout - 1 ))

  echo "test https://${TARGET}.${ZONE}"
  if curl -s "https://${TARGET}.${ZONE}"
  then
    echo "OK: https://${TARGET}.${ZONE}"
    break
  fi
  sleep 1
done
set -e

echo "provision loadtest client"
kubectl create -f ./loadtest/client

echo "load test started, waiting ${sleep_seconds} seconds"
sleep "$sleep_seconds"

kubectl get pods -n loadtest-e2e -o wide
echo "start-load-test.sh END"
