
import requests
import time

from clickclick import Action


def create_resource(manifest, url, token):
    response = requests.post(url, data=manifest,
                             headers={'Authorization': 'Bearer {}'.format(token), 'Content-Type': 'application/yaml'})
    response.raise_for_status()
    return response


def create_deployment(manifest, url, token):
    response = create_resource(manifest, url + '/apis/extensions/v1beta1/namespaces/e2e/deployments', token)
    return response


def wait_for_deployment(name, url, token):
    with Action('Waiting for deployment {}..'.format(name)):
        available = False
        for i in range(20):
            response = requests.get(url + '/apis/extensions/v1beta1/namespaces/e2e/deployments/{}'.format(name), headers={'Authorization': 'Bearer {}'.format(token)})
            data = response.json()
            if data.get('status', {}).get('availableReplicas') == 1:
                available = True
                break
            time.sleep(2)
    return available
