
import requests
import time

from clickclick import Action

DEPLOYMENTS_PATH = '/apis/extensions/v1beta1/namespaces/e2e/deployments'
SERVICES_PATH = '/api/v1/namespaces/e2e/services'
PETSET_PATH = '/apis/apps/v1alpha1/namespaces/e2e/petsets'
SECRET_PATH = '/api/v1/namespaces/e2e/secrets'
ENDPOINT_PATH = '/api/v1/namespaces/e2e/endpoints'
POD_PATH = '/api/v1/namespaces/e2e/pods'


def create_resource(manifest, url, token):
    response = requests.post(url, data=manifest,
                             headers={'Authorization': 'Bearer {}'.format(token), 'Content-Type': 'application/yaml'})
    response.raise_for_status()
    return response


def create_deployment(manifest, url, token):
    response = create_resource(manifest, url + DEPLOYMENTS_PATH, token)
    return response


def create_service(manifest, url, token):
    response = create_resource(manifest, url + SERVICES_PATH, token)
    return response


def wait_for_deployment(name, url, token, timeout=60):
    cut_off = time.time() + timeout
    with Action('Waiting for deployment {}..'.format(name)):
        available = False
        while time.time() < cut_off:
            response = requests.get(url + DEPLOYMENTS_PATH + '/{}'.format(name), headers={'Authorization': 'Bearer {}'.format(token)})
            data = response.json()
            if data.get('status', {}).get('availableReplicas') == 1:
                available = True
                break
            time.sleep(2)
    return available


def wait_for_pod(name, url, token, timeout=100):
    cut_off = time.time() + timeout
    with Action('Waiting for deployment {}..'.format(name)):
        available = False
        while time.time() < cut_off:
            response = requests.get(url + POD_PATH + '/{}'.format(name), headers={'Authorization': 'Bearer {}'.format(token)})
            data = response.json()
            if data.get('status', {}).get('phase') == 'Running':
                available = True
                break
            time.sleep(2)
    return available


def wait_for_resource(url, expected_content, timeout=60):
    cut_off = time.time() + timeout
    with Action('Waiting for resource {}..'.format(url)):
        available = False
        while time.time() < cut_off:
            try:
                response = requests.get(url, timeout=5)
                response.raise_for_status()
            except:
                # ignore any connection failures etc
                pass
            else:
                if expected_content in response.text:
                    available = True
                    break
            time.sleep(2)
    return available
