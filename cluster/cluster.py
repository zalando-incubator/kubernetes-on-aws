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


def encode_user_data(plain_text: str) -> str:
    b64_encoded = base64.b64encode(gzip.compress(plain_text.encode('utf-8')))
    return b64_encoded.decode('ascii')


def decode_user_data(user_data: str) -> str:
    return gzip.decompress(base64.b64decode(user_data.encode('ascii'))).decode('utf-8')


def get_user_data(fn, variables: dict) -> str:
    with open(fn) as fd:
        contents = fd.read()
    for key, value in variables.items():
        contents = contents.replace(key.upper(), value)
    return encode_user_data(contents)


def get_account_alias_without_namespace():
    '''
    Return AWS account alias without an optional prefix (separated by dash)

    i.e. an alias of "myorg-myteam-staging" will return "myteam-staging"
    '''
    iam = boto3.client('iam')
    account_alias = iam.list_account_aliases()['AccountAliases'][0]
    return account_alias.split('-', 1)[-1]


def get_cluster_variables(stack_name: str, version: str, worker_shared_secret=None):
    route53 = boto3.client('route53')
    all_hosted_zones = route53.list_hosted_zones()['HostedZones']
    hosted_zone = all_hosted_zones[0]['Name'].rstrip('.')
    account_alias_without_namespace = get_account_alias_without_namespace()

    etcd_discovery_domain = 'etcd.{}'.format(hosted_zone)
    api_server = 'https://{}-{}.{}'.format(stack_name, version, hosted_zone)
    if not worker_shared_secret:
        worker_shared_secret = ''.join(random.choice(string.ascii_letters + string.digits) for _ in range(64))
    cluster_name = '{}-{}'.format(account_alias_without_namespace, stack_name)
    # TODO: encrypt fixed token with KMS
    variables = {
        'stack_version': version,
        'etcd_discovery_domain': etcd_discovery_domain,
        'api_server': api_server,
        'worker_shared_secret': worker_shared_secret,
        'hosted_zone': hosted_zone,
        'webhook_cluster_name': cluster_name
    }
    return variables


def get_launch_configuration_user_data(stack_name, version):
    autoscaling = boto3.client('autoscaling')
    cf = boto3.client('cloudformation')
    resources = cf.describe_stack_resources(StackName='{}-{}'.format(stack_name, version))['StackResources']
    for resource in resources:
        if resource['ResourceType'] == 'AWS::AutoScaling::AutoScalingGroup' and 'Worker' in resource['LogicalResourceId']:
            asg_name = resource['PhysicalResourceId']
            break

    asgs = autoscaling.describe_auto_scaling_groups(AutoScalingGroupNames=[asg_name])['AutoScalingGroups']
    lc_name = asgs[0]['LaunchConfigurationName']
    lcs = autoscaling.describe_launch_configurations(LaunchConfigurationNames=[lc_name])['LaunchConfigurations']
    user_data = lcs[0]['UserData']
    return user_data


def get_worker_shared_secret(user_data: str):
    plain_text = decode_user_data(user_data)
    data = yaml.safe_load(plain_text)
    token = None
    for write_file in data['write_files']:
        if write_file['path'] == '/etc/kubernetes/worker-kubeconfig.yaml':
            kubeconfig = yaml.safe_load(write_file['content'])
            token = kubeconfig['users'][0]['user']['token']
    return token


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


def get_instances_to_update(stack_name, version, desired_user_data):
    autoscaling = boto3.client('autoscaling')
    cf = boto3.client('cloudformation')
    ec2 = boto3.client('ec2')
    resources = cf.describe_stack_resources(StackName='{}-{}'.format(stack_name, version))['StackResources']
    for resource in resources:
        if resource['ResourceType'] == 'AWS::AutoScaling::AutoScalingGroup' and 'Worker' in resource['LogicalResourceId']:
            asg_name = resource['PhysicalResourceId']
            break

    desired_plain_text = decode_user_data(desired_user_data)
    instance_ids = set()

    asgs = autoscaling.describe_auto_scaling_groups(AutoScalingGroupNames=[asg_name])['AutoScalingGroups']
    for instance in asgs[0]['Instances']:
        data = ec2.describe_instance_attribute(InstanceId=instance['InstanceId'], Attribute='userData')
        plain_text = decode_user_data(data['UserData']['Value'])
        if plain_text != desired_plain_text:
            instance_ids.add(instance['InstanceId'])

    return instance_ids


@cli.command()
@click.argument('stack_name')
@click.argument('version')
def update(stack_name, version):
    '''
    Update Kubernetes cluster
    '''
    user_data = get_launch_configuration_user_data(stack_name, version)
    worker_shared_secret = get_worker_shared_secret(user_data)
    variables = get_cluster_variables(stack_name, version, worker_shared_secret)
    userdata_master = get_user_data('userdata-master.yaml', variables)
    userdata_worker = get_user_data('userdata-worker.yaml', variables)

    if decode_user_data(user_data) == decode_user_data(userdata_worker):
        info('Worker user data did not change, not updating anything.')
        return

    # this will only update the Launch Configuration
    subprocess.check_call(['senza', 'update', 'senza-definition.yaml', version, 'StackName={}'.format(stack_name), 'UserDataMaster={}'.format(userdata_master), 'UserDataWorker={}'.format(userdata_worker), 'KmsKey=*'])
    # wait for CF update to complete..
    subprocess.check_call(['senza', 'wait', stack_name, version])
    # TODO: drain and respawn nodes
    instances_to_update = get_instances_to_update(stack_name, version, userdata_worker)
    print(instances_to_update)


@cli.command()
@click.argument('stack_name')
@click.argument('version')
def delete(stack_name, version):
    subprocess.check_call(['senza', 'delete', stack_name, version])


if __name__ == '__main__':
    cli()
