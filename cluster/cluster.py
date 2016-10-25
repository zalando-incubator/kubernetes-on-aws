#!/usr/bin/env python3

import base64
import boto3
import click
import gzip
import random
import requests
import subprocess
import string
import time
import yaml

from botocore.exceptions import ClientError
from clickclick import info, Action

SCALING_PROCESSES_TO_SUSPEND = ['AZRebalance', 'AlarmNotification', 'ScheduledActions']


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


def get_auto_scaling_group(stack_name, version, name_filter):
    cf = boto3.client('cloudformation')
    resources = cf.describe_stack_resources(StackName='{}-{}'.format(stack_name, version))['StackResources']
    for resource in resources:
        if resource['ResourceType'] == 'AWS::AutoScaling::AutoScalingGroup' and name_filter in resource['LogicalResourceId']:
            asg_name = resource['PhysicalResourceId']
            return asg_name


def get_launch_configuration_user_data(stack_name, version, name_filter):
    autoscaling = boto3.client('autoscaling')
    asg_name = get_auto_scaling_group(stack_name, version, name_filter)

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


def has_etcd_cluster():
    cf = boto3.client('cloudformation')
    try:
        cf.describe_stacks(StackName='etcd-cluster-etcd')
    except ClientError as err:
        response = err.response
        error_info = response['Error']
        error_message = error_info['Message']
        if 'does not exist' in error_message:
            return False
        else:
            raise
    return True


def deploy_etcd_cluster(hosted_zone):
    subprocess.check_call(['senza', 'create', 'etcd-cluster.yaml', 'etcd', 'HostedZone={}'.format(hosted_zone)])
    # wait up to 15m for stack to be created
    subprocess.check_call(['senza', 'wait', '--timeout=900', 'etcd-cluster', 'etcd'])


def tag_subnets():
    '''
    Tag all subnets with KubernetesCluster=kubernetes to make K8s AWS integration happy :-(
    '''
    ec2 = boto3.resource('ec2')
    for subnet in ec2.subnets.all():
        subnet.create_tags(Tags=[{'Key': 'KubernetesCluster',
                                  'Value': 'kubernetes'}])


def wait_for_api_server(api_server):
    with Action('Waiting for API server {}..'.format(api_server)) as act:
        while True:
            try:
                response = requests.get(api_server, timeout=5)
            except:
                response = None
            if response is not None and response.status_code == 401:
                return
            time.sleep(5)
            act.progress()


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
    if not has_etcd_cluster() and not dry_run:
        deploy_etcd_cluster(variables['hosted_zone'])
    tag_subnets()
    userdata_master = get_user_data('userdata-master.yaml', variables)
    userdata_worker = get_user_data('userdata-worker.yaml', variables)
    if not dry_run:
        subprocess.check_call(['senza', 'create', 'senza-definition.yaml', version, 'StackName={}'.format(stack_name), 'UserDataMaster={}'.format(userdata_master), 'UserDataWorker={}'.format(userdata_worker), 'KmsKey=*'])
        # wait up to 15m for stack to be created
        subprocess.check_call(['senza', 'wait', '--timeout=900', stack_name, version])
        wait_for_api_server(variables['api_server'])


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


def same_user_data(enc1, enc2):
    return decode_user_data(enc1) == decode_user_data(enc2)


@cli.command()
@click.argument('stack_name')
@click.argument('version')
@click.option('--force', is_flag=True)
def update(stack_name, version, force):
    '''
    Update Kubernetes cluster
    '''
    existing_user_data_master = get_launch_configuration_user_data(stack_name, version, 'Master')
    existing_user_data_worker = get_launch_configuration_user_data(stack_name, version, 'Worker')
    worker_shared_secret = get_worker_shared_secret(existing_user_data_worker)
    variables = get_cluster_variables(stack_name, version, worker_shared_secret)
    user_data_master = get_user_data('userdata-master.yaml', variables)
    user_data_worker = get_user_data('userdata-worker.yaml', variables)

    if not force and same_user_data(existing_user_data_master, user_data_master) and same_user_data(existing_user_data_worker, user_data_worker):
        info('Neither worker nor master user data did change, not updating anything.')
        return

    # this will only update the Launch Configuration
    subprocess.check_call(['senza', 'update', 'senza-definition.yaml', version, 'StackName={}'.format(stack_name),
                           'UserDataMaster={}'.format(user_data_master),
                           'UserDataWorker={}'.format(user_data_worker), 'KmsKey=*'])
    # wait for CF update to complete..
    subprocess.check_call(['senza', 'wait', '--timeout=600', stack_name, version])
    perform_node_updates(stack_name, version, 'Master', user_data_master)
    wait_for_api_server(variables['api_server'])
    perform_node_updates(stack_name, version, 'Worker', user_data_worker)


def perform_node_updates(stack_name, version, name_filter, desired_user_data):
    # TODO: only works for worker nodes right now
    autoscaling = boto3.client('autoscaling')
    asg_name = get_auto_scaling_group(stack_name, version, name_filter)

    with Action('Suspending scaling processes for {}..'.format(asg_name)):
        autoscaling.suspend_processes(AutoScalingGroupName=asg_name,
                                      ScalingProcesses=SCALING_PROCESSES_TO_SUSPEND)

    group = autoscaling.describe_auto_scaling_groups(AutoScalingGroupNames=[asg_name])['AutoScalingGroups'][0]
    old_desired_capacity = group['DesiredCapacity']
    new_desired_capacity = old_desired_capacity + 1
    # scale out
    with Action('Scaling up to {} instances..'.format(new_desired_capacity)) as act:
        autoscaling.update_auto_scaling_group(AutoScalingGroupName=asg_name,
                                              MinSize=new_desired_capacity,
                                              MaxSize=new_desired_capacity,
                                              DesiredCapacity=new_desired_capacity)
        while True:
            group = autoscaling.describe_auto_scaling_groups(AutoScalingGroupNames=[asg_name])['AutoScalingGroups'][0]
            instances_in_service = [inst for inst in group['Instances'] if inst['LifecycleState'] == 'InService']
            if len(instances_in_service) >= new_desired_capacity:
                break
            time.sleep(5)
            act.progress()

        # TODO: wait for nodes to be ready

    instances_to_update = get_instances_to_update(stack_name, version, desired_user_data)
    for instance_id in instances_to_update:
        # TODO: drain
        autoscaling = boto3.client('autoscaling')
        with Action('Terminating old instance {}..'.format(instance_id)):
            autoscaling.terminate_instance_in_auto_scaling_group(InstanceId=instance_id,
                                                                 ShouldDecrementDesiredCapacity=False)

    with Action('Scaling down to {} instances..'.format(old_desired_capacity)) as act:
        autoscaling.update_auto_scaling_group(AutoScalingGroupName=asg_name,
                                              MinSize=old_desired_capacity,
                                              MaxSize=old_desired_capacity,
                                              DesiredCapacity=old_desired_capacity)

    with Action('Resuming scaling processes for {}..'.format(asg_name)):
        autoscaling.resume_processes(AutoScalingGroupName=asg_name)


@cli.command()
@click.argument('stack_name')
@click.argument('version')
def delete(stack_name, version):
    subprocess.check_call(['senza', 'delete', stack_name, version])


if __name__ == '__main__':
    cli()
