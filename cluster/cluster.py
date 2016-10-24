#!/usr/bin/env python3

import base64
import boto3
import click
import gzip
import random
import subprocess
import string
import yaml

from clickclick import info


def get_user_data(fn, variables: dict) -> str:
    with open(fn) as fd:
        contents = fd.read()
    for key, value in variables.items():
        contents = contents.replace(key.upper(), value)
    b64_encoded = base64.b64encode(gzip.compress(contents.encode('utf-8')))
    return b64_encoded.decode('ascii')


def get_account_alias_without_namespace():
    '''
    Return AWS account alias without an optional prefix (separated by dash)

    i.e. an alias of "myorg-myteam" will return "myteam"
    '''
    iam = boto3.client('iam')
    account_alias = iam.list_account_aliases()['AccountAliases'][0]
    return account_alias.split('-')[-1]


def get_cluster_variables(stack_name: str, version: str):
    route53 = boto3.client('route53')
    all_hosted_zones = route53.list_hosted_zones()['HostedZones']
    hosted_zone = all_hosted_zones[0]['Name'].rstrip('.')
    account_alias_without_namespace = get_account_alias_without_namespace()

    etcd_discovery_domain = 'etcd.{}'.format(hosted_zone)
    api_server = 'https://{}-{}.{}'.format(stack_name, version, hosted_zone)
    token = ''.join(random.choice(string.ascii_letters + string.digits) for _ in range(64))
    cluster_name = '{}-{}'.format(account_alias_without_namespace, stack_name)
    # TODO: encrypt fixed token with KMS
    variables = {
        'stack_version': version,
        'etcd_discovery_domain': etcd_discovery_domain,
        'api_server': api_server,
        'worker_shared_secret': token,
        'hosted_zone': hosted_zone,
        'webhook_cluster_name': cluster_name
    }
    return variables


@click.group()
def cli():
    pass


@cli.command()
@click.argument('stack_name')
@click.argument('version')
@click.option('--dry-run', is_flag=True, help='No-op mode: show what would be created')
def create(stack_name, version, dry_run):
    '''
    Create a new Kubernetes cluster (using current AWS credentials)
    '''

    variables = get_cluster_variables(stack_name, version)
    info('Cluster name is:             {}'.format(variables['webhook_cluster_name']))
    info('API server endpoint will be: {}'.format(variables['api_server']))
    if dry_run:
        print(yaml.safe_dump(variables))
    userdata_master = get_user_data('userdata-master.yaml', variables)
    userdata_worker = get_user_data('userdata-worker.yaml', variables)
    if not dry_run:
        subprocess.check_call(['senza', 'create', 'senza-definition.yaml', version, 'StackName={}'.format(stack_name), 'UserDataMaster={}'.format(userdata_master), 'UserDataWorker={}'.format(userdata_worker), 'KmsKey=*'])


@cli.command()
@click.argument('stack_name')
@click.argument('version')
def update(stack_name, version):
    '''
    Update Kubernetes cluster
    '''
    variables = get_cluster_variables(stack_name, version)
    userdata_master = get_user_data('userdata-master.yaml', variables)
    userdata_worker = get_user_data('userdata-worker.yaml', variables)
    subprocess.check_call(['senza', 'update', 'senza-definition.yaml', version, 'StackName={}'.format(stack_name), 'UserDataMaster={}'.format(userdata_master), 'UserDataWorker={}'.format(userdata_worker), 'KmsKey=*'])


@cli.command()
@click.argument('stack_name')
@click.argument('version')
def delete(stack_name, version):
    subprocess.check_call(['senza', 'delete', stack_name, version])


if __name__ == '__main__':
    cli()
