#!/usr/bin/env python3
import argparse
import json
import subprocess


def toggle_scaledown(enabled):
    # Try to retrieve daemonset/kube-cluster-autoscalerfrom the e2e cluster
    cmd_output = ""
    try:
        cmd_output = subprocess.check_output(
            ["kubectl", "get", "daemonset", "-o", "json", "-n", "kube-system", "kube-cluster-autoscaler"],
            stderr=subprocess.STDOUT)
    except subprocess.CalledProcessError as e:
        # This happens when a cluster has been cleaned up, so we should handle it gracefully
        if "no such host" in e.output.decode("utf-8"):
            print("Failed to reach the API server, is the cluster running?")
        raise e
    
    current = json.loads(cmd_output.decode("utf-8"))
    for i, container in enumerate(current["spec"]["template"]["spec"]["containers"]):
        if container["name"] == "cluster-autoscaler":
            command = container["command"]
            updated_arg = "--scale-down-enabled={}".format("true" if enabled else "false")
            updated_command = [updated_arg if "scale-down-enabled" in arg else arg for arg in command]
            if command != updated_command:
                patch = [
                    {
                        "op": "replace",
                        "path": "/spec/template/spec/containers/{}/command".format(i),
                        "value": updated_command,
                    }
                ]
                subprocess.check_call(
                    [
                        "kubectl",
                        "patch",
                        "daemonset",
                        "-n",
                        "kube-system",
                        "kube-cluster-autoscaler",
                        "--type=json",
                        "-p",
                        json.dumps(patch),
                    ]
                )


def main():
    parser = argparse.ArgumentParser(description="Enable or disable scale-down.")
    parser.add_argument(
        "action", help="Whether scale-down should be enabled or disabled.", choices=["enable", "disable"]
    )
    args = parser.parse_args()

    enabled = args.action == "enable"
    
    try:
        toggle_scaledown(enabled)
    except subprocess.CalledProcessError as e:
        print("Failed to toggle scale-down.")
        exit(1)

if __name__ == "__main__":
    main()
