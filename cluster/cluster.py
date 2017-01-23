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
from clickclick import info, Action, warning

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
        contents = contents.replace('{{' + key.upper() + '}}', value)
    return encode_user_data(contents)


def get_account_id():
    conn = boto3.client('iam')
    try:
        own_user = conn.get_user()['User']
    except:
        own_user = None
    if not own_user:
        roles = conn.list_roles()['Roles']
        if not roles:
            users = conn.list_users()['Users']
            if not users:
                saml = conn.list_saml_providers()['SAMLProviderList']
                if not saml:
                    return None
                else:
                    arn = [s['Arn'] for s in saml][0]
            else:
                arn = [u['Arn'] for u in users][0]
        else:
            arn = [r['Arn'] for r in roles][0]
    else:
        arn = own_user['Arn']
    account_id = arn.split(':')[4]
    return account_id


def get_account_alias():
    conn = boto3.client('iam')
    return conn.list_account_aliases()['AccountAliases'][0]


def get_mint_bucket_name():
    account_id = get_account_id()
    account_alias = get_account_alias()
    s3 = boto3.resource('s3')
    parts = account_alias.split('-')
    prefix = parts[0]
    region = get_region()
    bucket_name = '{}-stups-mint-{}-{}'.format(prefix, account_id, region)
    bucket = s3.Bucket(bucket_name)
    try:
        bucket.load()
        return bucket.name
    except:
        bucket = None
    for bucket in s3.buckets.all():
        if bucket.name.startswith('{}-stups-mint-{}-'.format(prefix, account_id)):
            return bucket.name
    return bucket_name


def get_region():
    my_session = boto3.session.Session()
    return my_session.region_name


def get_etcd_bucket_name():
    account_id = get_account_id()
    account_alias = get_account_alias()
    region = get_region()
    bucket_name = '{}-etcd-{}-{}'.format(account_alias, account_id, region)
    return bucket_name


def get_cluster_variables(stack_name: str, version: str, node_labels: str, scalyr_access_key: str, worker_shared_secret=None):
    route53 = boto3.client('route53')
    all_hosted_zones = route53.list_hosted_zones()['HostedZones']
    hosted_zone = all_hosted_zones[0]['Name'].rstrip('.')

    etcd_discovery_domain = 'etcd.{}'.format(hosted_zone)
    api_server = 'https://{}-{}.{}'.format(stack_name, version, hosted_zone)
    if not worker_shared_secret:
        worker_shared_secret = ''.join(random.choice(string.ascii_letters + string.digits) for _ in range(64))

    cluster_id = 'aws:{}:{}:{}-{}'.format(get_account_id(), get_region(), stack_name, version)
    webhook_id = cluster_id

    if stack_name == "kube-aws-test":
        webhook_id = 'aws:{}:{}:{}'.format(get_account_id(), get_region(), stack_name)

    # TODO: encrypt fixed token with KMS

    mint_bucket = get_mint_bucket_name()
    etcd_bucket = get_etcd_bucket_name()

    variables = {
        'stack_name': stack_name,
        'stack_version': version,
        'etcd_discovery_domain': etcd_discovery_domain,
        'api_server': api_server,
        'worker_shared_secret': worker_shared_secret,
        'hosted_zone': hosted_zone,
        'cluster_id': cluster_id,
        'webhook_id': webhook_id,
        'mint_bucket': mint_bucket,
        'etcd_bucket': etcd_bucket,
        'account_id': get_account_id(),
        'region': get_region(),
        'node_labels': node_labels,
        'scalyr_access_key': scalyr_access_key
    }
    return variables


def get_auto_scaling_group(stack_name, version, name_filter):
    cf = boto3.client('cloudformation')
    resources = cf.describe_stack_resources(StackName='{}-{}'.format(stack_name, version))['StackResources']
    for resource in resources:
        if resource['ResourceType'] == 'AWS::AutoScaling::AutoScalingGroup' and name_filter in resource['LogicalResourceId']:
            asg_name = resource['PhysicalResourceId']
            return asg_name


def get_launch_configuration(stack_name, version, name_filter):
    autoscaling = boto3.client('autoscaling')
    asg_name = get_auto_scaling_group(stack_name, version, name_filter)

    asgs = autoscaling.describe_auto_scaling_groups(AutoScalingGroupNames=[asg_name])['AutoScalingGroups']
    lc_name = asgs[0]['LaunchConfigurationName']
    lcs = autoscaling.describe_launch_configurations(LaunchConfigurationNames=[lc_name])['LaunchConfigurations']
    return lcs[0]


def get_launch_configuration_user_data(stack_name, version, name_filter):
    return get_launch_configuration(stack_name, version, name_filter)['UserData']


def get_current_nodes(stack_name, version, name_filter, key='DesiredCapacity'):
    autoscaling = boto3.client('autoscaling')
    asg_name = get_auto_scaling_group(stack_name, version, name_filter)

    group = autoscaling.describe_auto_scaling_groups(AutoScalingGroupNames=[asg_name])['AutoScalingGroups'][0]
    return group[key]


def get_current_master_nodes(stack_name, version):
    return get_current_nodes(stack_name, version, 'Master')


def get_current_worker_nodes(stack_name, version):
    return get_current_nodes(stack_name, version, 'Worker')


def get_current_min_worker_nodes(stack_name, version):
    return get_current_nodes(stack_name, version, 'Worker', 'MinSize')


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


def deploy_etcd_cluster(hosted_zone, etcd_bucket, region):
    s3 = boto3.resource('s3', region)
    with Action('Checking S3 bucket {}..'.format(etcd_bucket)) as act:
        bucket = s3.Bucket(etcd_bucket)
        try:
            bucket.creation_date
        except:
            act.warning('creating bucket..')
            bucket.create(CreateBucketConfiguration={'LocationConstraint': region})
            bucket.wait_until_exists()

    subprocess.check_call(['senza', 'create', 'etcd-cluster.yaml', 'etcd', 'HostedZone={}'.format(hosted_zone), 'EtcdS3Backup={}'.format(etcd_bucket)])
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
    with Action('Waiting for API server {} ..'.format(api_server)) as act:
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
@click.option('--instance-type', type=str, default='t2.micro', help='Type of instance')
@click.option('--master-nodes', default=1, type=int, help='Number of master nodes')
@click.option('--worker-nodes', default=1, type=int, help='Number of worker nodes')
@click.option('--min-worker-nodes', default=1, type=int, help='Minimum number of nodes in the worker ASG')
@click.option('--max-worker-nodes', default=10, type=int, help='Maximum number of nodes in the worker ASG')
@click.option('--node-labels', type=str, default='', help='Labels to assign to each node')
@click.option('--scalyr-access-key', type=str, required=True, help='Secret for the logging agent')
def create(stack_name, version, dry_run, instance_type, master_nodes, worker_nodes, min_worker_nodes, max_worker_nodes, node_labels, scalyr_access_key):
    '''
    Create a new Kubernetes cluster (using current AWS credentials)
    '''

    variables = get_cluster_variables(stack_name, version, node_labels, scalyr_access_key)
    info('Cluster ID is:               {}'.format(variables['cluster_id']))
    info('API server endpoint will be: {}'.format(variables['api_server']))
    if dry_run:
        print(yaml.safe_dump(variables))
    # TODO: register mint bucket with "kube-secretary" app
    if not has_etcd_cluster() and not dry_run:
        deploy_etcd_cluster(variables['hosted_zone'], variables['etcd_bucket'], variables['region'])
    tag_subnets()
    userdata_master = get_user_data('userdata-master.yaml', variables)
    userdata_worker = get_user_data('userdata-worker.yaml', variables)
    if not dry_run:
        subprocess.check_call(['senza', 'create', 'senza-definition.yaml', version, 'StackName={}'.format(stack_name),
                               'UserDataMaster={}'.format(userdata_master), 'UserDataWorker={}'.format(userdata_worker), 'KmsKey=*',
                               'MasterNodes={}'.format(master_nodes), 'WorkerNodes={}'.format(worker_nodes),
                               'MinimumWorkerNodes={}'.format(min_worker_nodes),
                               'MaximumWorkerNodes={}'.format(max_worker_nodes),
                               'HostedZone={}'.format(variables['hosted_zone']),
                               'InstanceType={}'.format(instance_type), 'ClusterID={}'.format(variables['cluster_id'])])
        # wait up to 15m for stack to be created
        subprocess.check_call(['senza', 'wait', '--timeout=900', stack_name, version])
        wait_for_api_server(variables['api_server'])


def parse_label_string(label_string) -> dict:
    labels = dict()

    if label_string == "":
        return labels

    splitted_labels = label_string.split(",")
    for label in splitted_labels:
        key, value = label.split("=", 2)
        labels[key] = value

    return labels


def node_matches_labels(node, labels) -> bool:
    node_labels = node["metadata"]["labels"]
    for key in labels.keys():
        label_matches = key in node_labels and node_labels[key] == labels[key]
        if not label_matches:
            return False

    return True


def get_instances_to_update_by_labels(desired_node_labels, config):
    instance_ids = set()

    parsed_labels = parse_label_string(desired_node_labels)

    nodes = get_k8s_nodes(config["api_server"], config["worker_shared_secret"])
    for node in nodes:
        if not node_matches_labels(node, parsed_labels):
            instance_ids.add(node["spec"]["externalID"])

    return instance_ids


def same_user_data(enc1, enc2):
    return decode_user_data(enc1) == decode_user_data(enc2)


def get_instances_to_update_by_userdata(asg_name, desired_user_data):
    autoscaling = boto3.client('autoscaling')
    ec2 = boto3.client('ec2')

    desired_plain_text = decode_user_data(desired_user_data)
    instance_ids = set()

    asgs = autoscaling.describe_auto_scaling_groups(AutoScalingGroupNames=[asg_name])['AutoScalingGroups']
    for instance in asgs[0]['Instances']:
        data = ec2.describe_instance_attribute(InstanceId=instance['InstanceId'], Attribute='userData')
        plain_text = decode_user_data(data['UserData']['Value'])
        if plain_text != desired_plain_text:
            instance_ids.add(instance['InstanceId'])

    return instance_ids


def get_instances_to_update(asg_name, desired_user_data, desired_node_labels, config):
    by_userdata = get_instances_to_update_by_userdata(asg_name, desired_user_data)
    by_labels = get_instances_to_update_by_labels(desired_node_labels, config)

    return by_userdata.union(by_labels)


@cli.command()
@click.argument('stack_name')
@click.argument('version')
@click.option('--dry-run', is_flag=True, help='No-op mode: show what would be created')
@click.option('--force', is_flag=True)
@click.option('--instance-type', type=str, default='current', help='Type of instance')
@click.option('--master-nodes', type=int, default=-1, help='Number of master nodes')
@click.option('--worker-nodes', type=int, default=-1, help='Number of worker nodes')
@click.option('--postpone', is_flag=True, help='Postpone node update to a later point in time')
@click.option('--min-worker-nodes', default=-1, type=int, help='Minimum number of nodes in the worker ASG')
@click.option('--max-worker-nodes', default=10, type=int, help='Maximum number of nodes in the worker ASG')
@click.option('--node-labels', type=str, default='', help='Labels to assign to each node')
@click.option('--scalyr-access-key', type=str, required=True, help='Secret for the logging agent')
def update(stack_name, version, dry_run, force, instance_type, master_nodes, worker_nodes, postpone, min_worker_nodes, max_worker_nodes, node_labels, scalyr_access_key):
    '''
    Update Kubernetes cluster
    '''
    existing_user_data_master = get_launch_configuration_user_data(stack_name, version, 'Master')
    existing_user_data_worker = get_launch_configuration_user_data(stack_name, version, 'Worker')
    worker_shared_secret = get_worker_shared_secret(existing_user_data_worker)
    variables = get_cluster_variables(stack_name, version, node_labels, scalyr_access_key, worker_shared_secret)
    if dry_run:
        print(yaml.safe_dump(variables))
    user_data_master = get_user_data('userdata-master.yaml', variables)
    user_data_worker = get_user_data('userdata-worker.yaml', variables)

    if instance_type == 'current':
        instance_type = get_launch_configuration(stack_name, version, 'Worker')['InstanceType']

    if not force and same_user_data(existing_user_data_master, user_data_master) and same_user_data(existing_user_data_worker, user_data_worker):
        info('Neither worker nor master user data did change, not updating anything.')
        return

    if master_nodes == -1:
        master_nodes = get_current_master_nodes(stack_name, version)

    if worker_nodes == -1:
        worker_nodes = get_current_worker_nodes(stack_name, version)

    if min_worker_nodes == -1:
        min_worker_nodes = get_current_min_worker_nodes(stack_name, version)

    info('Cluster ID is:          {}'.format(variables['cluster_id']))
    info('API server endpoint is: {}'.format(variables['api_server']))
    info('Master nodes:           {}'.format(master_nodes))
    info('Worker nodes:           {}, min: {}, max: {}'.format(worker_nodes, min_worker_nodes, max_worker_nodes))
    info('Instance type:          {}'.format(instance_type))

    if not dry_run:
        # this will only update the Launch Configuration
        subprocess.check_call(['senza', 'update', 'senza-definition.yaml', version, 'StackName={}'.format(stack_name),
                               'UserDataMaster={}'.format(user_data_master),
                               'UserDataWorker={}'.format(user_data_worker), 'KmsKey=*',
                               'MasterNodes={}'.format(master_nodes), 'WorkerNodes={}'.format(worker_nodes),
                               'MinimumWorkerNodes={}'.format(min_worker_nodes),
                               'MaximumWorkerNodes={}'.format(max_worker_nodes),
                               'HostedZone={}'.format(variables['hosted_zone']),
                               'InstanceType={}'.format(instance_type), 'ClusterID={}'.format(variables['cluster_id'])])
        # wait for CF update to complete..
        subprocess.check_call(['senza', 'wait', '--timeout=600', stack_name, version])

        if not postpone:
            perform_node_updates(stack_name, version, 'Master', user_data_master, node_labels, variables)
            wait_for_api_server(variables['api_server'])
            perform_node_updates(stack_name, version, 'Worker', user_data_worker, node_labels, variables)


def get_k8s_nodes(api_server: str, token: str) -> list:
    headers = {"Authorization": "Bearer {}".format(token)}
    try:
        response = requests.get(api_server + "/api/v1/nodes", headers=headers, timeout=5)
        response.raise_for_status()
        return response.json()['items']
    except Exception as e:
        warning('Failed to query API server for nodes: {}'.format(e))
        return []


def get_k8s_node_name(instance_id: str, config: dict):
    nodes = get_k8s_nodes(config["api_server"], config["worker_shared_secret"])
    for node in nodes:
        if node["spec"]["externalID"] == instance_id:
            return node["metadata"]["name"]
    return ""


def longest_grace_period(node_name: str, config: dict):
    """
    Find the longest grace period of any pods on the node.
    """
    headers = {"Authorization": "Bearer {}".format(config["worker_shared_secret"])}
    params = {"fieldSelector": "spec.nodeName={}".format(node_name)}
    pods = requests.get(config["api_server"] + "/api/v1/pods", params=params, headers=headers, timeout=5).json()
    grace_period = 0
    for pod in pods["items"]:
        grace_period = max(pod["spec"]["terminationGracePeriodSeconds"], grace_period)
    return grace_period


def drain_node(node_name: str, config: dict, max_grace_period=60):
    """
    Drains a node for pods. Pods will be terminated with a grace period
    respecting the longest grace period of any pod on the node limited to
    max_grace_period. Default max_grace_period is 60s.
    """
    # respect pod terminate grace period
    grace_period = min(longest_grace_period(node_name, config), max_grace_period)

    for i in range(3):
        try:
            subprocess.check_call([
                'kubectl',
                '--server', config["api_server"],
                '--token', config["worker_shared_secret"],
                'drain', node_name,
                '--force', '--delete-local-data',
                '--ignore-daemonsets',
                '--grace-period={}'.format(grace_period)])
            break
        except Exception as e:
            warning('Kubectl failed to drain node: {}'.format(e))
            time.sleep(grace_period)
    time.sleep(grace_period)


def k8s_node_ready(instance_id: str, nodes: list):
    for node in nodes:
        if node["spec"]["externalID"] == instance_id:
            for cond in node["status"]["conditions"]:
                if cond["type"] == "Ready" and cond["status"] == "True":
                    return True
    return False


def get_instances_in_service(group: dict, config: dict):
    # this only handles classic ELB (ELBv1)
    instances_in_service = set()
    lb_names = group['LoadBalancerNames']
    if lb_names:
        # check ELB status
        elb = boto3.client('elb')
        for lb_name in lb_names:
            result = elb.describe_instance_health(LoadBalancerName=lb_name)
            for instance in result['InstanceStates']:
                if instance['State'] == 'InService':
                    instances_in_service.add(instance['InstanceId'])
    else:
        # use ASG LifecycleState and check that node is Ready in k8s
        autoscaling = boto3.client('autoscaling')
        group = autoscaling.describe_auto_scaling_groups(AutoScalingGroupNames=[group['AutoScalingGroupName']])['AutoScalingGroups'][0]
        # get k8s nodes
        nodes = get_k8s_nodes(config["api_server"], config["worker_shared_secret"])
        for instance in group['Instances']:
            if instance['LifecycleState'] == 'InService':
                if k8s_node_ready(instance["InstanceId"], nodes):
                    instances_in_service.add(instance['InstanceId'])
    return instances_in_service


def perform_node_updates(stack_name, version, name_filter, desired_user_data, desired_node_labels, config):
    # TODO: only works for worker nodes right now
    autoscaling = boto3.client('autoscaling')
    asg_name = get_auto_scaling_group(stack_name, version, name_filter)

    with Action('Suspending scaling processes for {}..'.format(asg_name)):
        autoscaling.suspend_processes(AutoScalingGroupName=asg_name,
                                      ScalingProcesses=SCALING_PROCESSES_TO_SUSPEND)

    group = autoscaling.describe_auto_scaling_groups(AutoScalingGroupNames=[asg_name])['AutoScalingGroups'][0]
    old_desired_capacity = group['DesiredCapacity']
    old_min_size = group['MinSize']
    old_max_size = group['MaxSize']
    new_desired_capacity = old_desired_capacity + 1
    # scale out
    with Action('Scaling up to {} instances..'.format(new_desired_capacity)) as act:
        autoscaling.update_auto_scaling_group(AutoScalingGroupName=asg_name,
                                              MinSize=new_desired_capacity,
                                              MaxSize=new_desired_capacity,
                                              DesiredCapacity=new_desired_capacity)
        while True:
            instances_in_service = get_instances_in_service(group, config)
            if len(instances_in_service) >= new_desired_capacity:
                break
            time.sleep(5)
            act.progress()

    instances_to_update = get_instances_to_update(asg_name, desired_user_data, desired_node_labels, config)
    for instance_id in instances_to_update:
        # drain
        node_name = get_k8s_node_name(instance_id, config)
        drain_node(node_name, config)

        autoscaling = boto3.client('autoscaling')
        with Action('Terminating old instance {}..'.format(instance_id)):
            autoscaling.terminate_instance_in_auto_scaling_group(InstanceId=instance_id,
                                                                 ShouldDecrementDesiredCapacity=False)
            while True:
                instances_in_service = get_instances_in_service(group, config)
                if instances_in_service and instance_id not in instances_in_service:
                    break
                time.sleep(2)
                act.progress()

        with Action('Waiting for ASG to scale back to {} instances..'.format(new_desired_capacity)) as act:
            while True:
                instances_in_service = get_instances_in_service(group, config)
                if len(instances_in_service) >= new_desired_capacity:
                    break
                time.sleep(5)
                act.progress()

    with Action('Scaling down to {} instances..'.format(old_desired_capacity)) as act:
        autoscaling.update_auto_scaling_group(AutoScalingGroupName=asg_name,
                                              MinSize=old_min_size,
                                              MaxSize=old_max_size,
                                              DesiredCapacity=old_desired_capacity)

    with Action('Resuming scaling processes for {}..'.format(asg_name)):
        autoscaling.resume_processes(AutoScalingGroupName=asg_name)


@cli.command()
@click.argument('stack_name')
@click.argument('version')
def delete(stack_name, version):
    '''
    Tear down a whole Kubernetes cluster
    '''
    subprocess.check_call(['senza', 'delete', stack_name, version])


@cli.command('get-api-token')
@click.argument('stack_name')
@click.argument('version')
def get_api_token(stack_name, version):
    '''
    Return Bearer token for API server
    '''
    existing_user_data_worker = get_launch_configuration_user_data(stack_name, version, 'Worker')
    worker_shared_secret = get_worker_shared_secret(existing_user_data_worker)
    print(worker_shared_secret)


if __name__ == '__main__':
    cli()
