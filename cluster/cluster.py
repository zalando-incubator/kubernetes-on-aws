#!/usr/bin/env python3

import boto3
import click
import random
import subprocess
import string


def get_user_data(fn, variables: dict):
    with open(fn) as fd:
        contents = fd.read()
    for key, value in variables.items():
        contents = contents.replace(key.upper(), value)
    return contents


@click.group()
def cli():
    pass


@cli.command()
@click.argument('cluster_name')
@click.argument('version')
def create(cluster_name, version):
    route53 = boto3.client('route53')
    all_hosted_zones = route53.list_hosted_zones()['HostedZones']
    hosted_zone = all_hosted_zones[0]['Name'].rstrip('.')
    print(hosted_zone)
    etcd_discovery_domain = 'etcd.{}'.format(hosted_zone)
    api_server = '{}-{}.{}'.format(cluster_name, version, hosted_zone)
    token = ''.join(random.choice(string.ascii_letters + string.digits) for _ in range(64))
    print(token)
    variables = {
        'stack_version': version,
        'etcd_discovery_domain': etcd_discovery_domain,
        'api_server': api_server,
        'worker_shared_secret': token,
        'hosted_zone': hosted_zone
    }

    userdata_master = get_user_data('userdata-master.yaml', variables)
    userdata_worker = get_user_data('userdata-worker.yaml', variables)
    subprocess.check_call(['senza', 'create', 'senza-definition.yaml', version, 'UserDataMaster={}'.format(userdata_master), 'UserDataWorker={}'.format(userdata_worker), 'KmsKey=unused'])


@cli.command()
def update():
    pass


@cli.command()
def delete():
    pass


if __name__ == '__main__':
    cli()
