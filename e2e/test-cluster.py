#!/usr/bin/env python3

import click
import importlib
import os
import random
import requests
import string
import time

from clickclick import Action, fatal_error, info

from tests.helpers import create_resource


EXPECTED_CONTAINERS = frozenset([
    'kube-scheduler', 'nginx', 'webhook', 'healthz', 'heapster-nanny', 'kubernetes-dashboard',
    'kubedns', 'kube-apiserver', 'kube-controller-manager', 'mate', 'secretary', 'kubectl', 'kube-proxy',
    'dnsmasq', 'gerry', 'heapster', 'prometheus-node-exporter'])


def get_containers(url, token):
    headers = {"Authorization": "Bearer {}".format(token)}
    pods = requests.get(url + "/api/v1/pods", headers=headers, timeout=5).json()
    containers = {}
    for pod in pods['items']:
        statuses = pod.get('status', {}).get('containerStatuses', [])
        for container in statuses:
            containers[container['name']] = {'restart_count': container['restartCount'], 'ready': container['ready'], 'state': container['state']}
    return containers


@click.command()
@click.argument('url')
@click.option('--token')
def main(url, token):

    run_id = ''.join(random.choice(string.ascii_lowercase + string.digits) for _ in range(8))
    info('Starting test run {}..'.format(run_id))

    all_containers_ready = False

    with Action('Waiting for all containers to be ready..') as act:
        for i in range(30):
            containers = get_containers(url, token)
            ready = True
            for name in EXPECTED_CONTAINERS:
                if not containers.get(name, {}).get('ready'):
                    info('{} is not ready yet (restarts: {})'.format(name, containers.get(name, {}).get('restart_count')))
                    ready = False

            if ready:
                all_containers_ready = True
                break

            time.sleep(5)
            act.progress()

    if not all_containers_ready:
        fatal_error('Not all containers are ready')

    manifest = '''
apiVersion: v1
kind: Namespace
metadata:
    name: e2e
'''
    try:
        create_resource(manifest, url + '/api/v1/namespaces', token)
    except requests.exceptions.HTTPError as e:
        # it's ok if the namespace is already there (409 Conflict)
        if e.response.status_code != 409:
            raise

    for entry in os.listdir('tests'):
        if entry.startswith('test_'):
            module_name = entry.split('.')[0]
            module = importlib.import_module('tests.{}'.format(module_name))
            func = getattr(module, module_name)
            info('Running {}..'.format(module_name))
            func(run_id, url, token)


if __name__ == '__main__':
    main()
