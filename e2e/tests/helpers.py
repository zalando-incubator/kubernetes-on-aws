
import requests
import time

from clickclick import Action

DEPLOYMENTS_PATH = '/apis/extensions/v1beta1/namespaces/e2e/deployments'
SERVICES_PATH = '/api/v1/namespaces/e2e/services'


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


def wait_for_endpoint(url, expected_content, timeout=60):
    cut_off = time.time() + timeout
    with Action('Waiting for endpoint {}..'.format(url)):
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
