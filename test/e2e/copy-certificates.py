#!/usr/bin/env python3
import argparse
import yaml


def read_cluster(filename):
    with open(filename, "r") as f:
        data = yaml.safe_load(f)
        if len(data["clusters"]) != 1:
            raise Exception("Expected to find 1 cluster, found {}".format(len(data["clusters"])))
        return data["clusters"][0]


def main():
    parser = argparse.ArgumentParser(description="Copy the certificate-related config items between two cluster YAMLs.")
    parser.add_argument("source", help="Source YAML file.")
    parser.add_argument("destination", help="Destination YAML file.")
    args = parser.parse_args()

    source_cluster = read_cluster(args.source)
    destination_cluster = read_cluster(args.destination)

    for k, v in source_cluster["config_items"].items():
        if k in destination_cluster["config_items"]:
            continue
        if (
            k.endswith("_cert")
            or k.endswith("_cert_decompressed")
            or k.endswith("_key")
            or k.endswith("_key_decompressed")
        ):
            destination_cluster["config_items"][k] = v

    with open(args.destination, "w") as f:
        result = {"clusters": [destination_cluster]}
        f.write(yaml.dump(result, default_flow_style=False))


if __name__ == "__main__":
    main()
