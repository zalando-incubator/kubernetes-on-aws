#!/usr/bin/env python3

import base64
import boto3
import click
import gzip
import random
import subprocess
import string


def get_user_data(fn, variables: dict) -> str:
    with open(fn) as fd:
        contents = fd.read()
    for key, value in variables.items():
        contents = contents.replace(key.upper(), value)
    b64_encoded = base64.b64encode(gzip.compress(contents.encode('utf-8')))
    return b64_encoded.decode('ascii')


@click.group()
def cli():
    pass


@cli.command()
@click.argument('cluster_name')
@click.argument('version')
def create(cluster_name, version):
    '''
    Create a new Kubernetes cluster (using current AWS credentials)
    '''
    route53 = boto3.client('route53')
    all_hosted_zones = route53.list_hosted_zones()['HostedZones']
    hosted_zone = all_hosted_zones[0]['Name'].rstrip('.')
    etcd_discovery_domain = 'etcd.{}'.format(hosted_zone)
    api_server = 'https://{}-{}.{}'.format(cluster_name, version, hosted_zone)
    token = ''.join(random.choice(string.ascii_letters + string.digits) for _ in range(64))
    # TODO: encrypt fixed token with KMS
    variables = {
        'stack_version': version,
        'etcd_discovery_domain': etcd_discovery_domain,
        'api_server': api_server,
        'worker_shared_secret': token,
        'hosted_zone': hosted_zone
    }

    userdata_master = get_user_data('userdata-master.yaml', variables)
    userdata_worker = get_user_data('userdata-worker.yaml', variables)
    subprocess.check_call(['senza', 'create', 'senza-definition.yaml', version, 'StackName={}'.format(cluster_name), 'UserDataMaster={}'.format(userdata_master), 'UserDataWorker={}'.format(userdata_worker), 'KmsKey=*'])


@cli.command()
@click.argument('cluster_name')
@click.argument('version')
def update(cluster_name, version):
    '''
    Update Kubernetes cluster
    '''
    pass


@cli.command()
@click.argument('cluster_name')
@click.argument('version')
def delete(cluster_name, version):
    subprocess.check_call(['senza', 'delete', cluster_name, version])


if __name__ == '__main__':
    cli()
