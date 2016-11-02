
import requests
import time

from clickclick import fatal_error, Action


def test_nginx(run_id, url, token):
    manifest = '''
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: nginx-{run_id}
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: nginx-{run_id}
    spec:
      containers:
      - name: nginx
        image: nginx
        ports:
        - containerPort: 80
'''.format(run_id=run_id)
    response = requests.post(url + '/apis/extensions/v1beta1/namespaces/e2e/deployments', data=manifest, headers={'Authorization': 'Bearer {}'.format(token), 'Content-Type': 'application/yaml'})
    response.raise_for_status()

    with Action('Waiting for deployment of nginx..'):
        available = False
        for i in range(20):
            response = requests.get(url + '/apis/extensions/v1beta1/namespaces/e2e/deployments/nginx-{}'.format(run_id), headers={'Authorization': 'Bearer {}'.format(token)})
            data = response.json()
            if data.get('status', {}).get('availableReplicas') == 1:
                available = True
                break
            time.sleep(2)

    if not available:
        fatal_error('Deployment failed')
