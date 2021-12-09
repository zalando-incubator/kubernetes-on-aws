#!/usr/bin/env python3
import argparse
import json
import logging
import subprocess
import sys
import time
from datetime import datetime, timedelta


def fetch_objects(kind):
    output = subprocess.check_output(["kubectl", "get", kind, "-o", "json", "--all-namespaces"])
    for item in json.loads(output.decode("utf-8"))["items"]:
        key = "{}/{}".format(item["metadata"]["namespace"], item["metadata"]["name"])
        yield key, item


def update_complete(kind, check_fn):
    check_results = {name: check_fn(obj["spec"], obj.get("status", {})) for name, obj in fetch_objects(kind)}
    pending_update = {name: check_result for name, check_result in check_results.items() if check_result}
    for elem, check_result in pending_update.items():
        logging.info("Still updating: {} {}, {}".format(kind, elem, check_result))
    return len(pending_update) == 0


def daemonset_updated(spec, status):
    desired = status.get("desiredNumberScheduled", 0)
    ready = status.get("numberReady", 0)
    if desired == ready:
        return None
    else:
        return "{}/{} [{}]".format(ready, desired, condition_messages(status))


def deployment_updated(spec, status):
    replicas = spec.get("replicas", 1)
    ready = status.get("readyReplicas", 0)
    updated = status.get("updatedReplicas", 0)
    if updated != replicas:
        return "{}/{} updated [{}]".format(updated, replicas, condition_messages(status))
    if ready != replicas:
        return "{}/{} ready [{}]".format(ready, replicas, condition_messages(status))


def statefulset_updated(spec, status):
    replicas = spec.get("replicas", 1)
    ready = status.get("readyReplicas", 0)
    current = status.get("currentReplicas", 0)
    if current != replicas:
        return "{}/{} current [{}]".format(current, replicas, condition_messages(status))
    if ready != replicas:
        return "{}/{} ready [{}]".format(ready, replicas, condition_messages(status))


def condition_messages(status):
    return ' '.join([c.get('message', '') for c in status.get('conditions', [])])


def main():
    logging.basicConfig(level=logging.INFO, format="%(levelname)s [%(asctime)s]: %(message)s")

    parser = argparse.ArgumentParser(description='Wait for Kubernetes resources to be up-to-date and ready.')
    parser.add_argument('--timeout', type=int, dest='timeout', required=True, help="How long to wait (in seconds).")
    args = parser.parse_args()

    deadline = datetime.now() + timedelta(seconds=args.timeout)

    while datetime.now() < deadline:
        try:
            if all([update_complete("daemonset", daemonset_updated),
                    update_complete("deployment", deployment_updated),
                    update_complete("statefulset", statefulset_updated)]):
                sys.exit(0)
        except subprocess.CalledProcessError:
            logging.info("kubectl failed, will retry...")
        time.sleep(5)

    logging.error("Timeout exceeded!")
    sys.exit(124)


if __name__ == "__main__":
    main()
