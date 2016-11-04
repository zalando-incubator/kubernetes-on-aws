import urllib.parse
from clickclick import fatal_error

from .helpers import wait_for_deployment, create_deployment, create_service, wait_for_endpoint


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
    create_deployment(manifest, url, token)

    manifest = '''
kind: Service
apiVersion: v1
metadata:
  name: nginx-{run_id}
spec:
  selector:
    app: nginx-{run_id}
  type: LoadBalancer
  ports:
    - port: 80
      targetPort: 80
      protocol: TCP
      name: http
'''.format(run_id=run_id)
    create_service(manifest, url, token)

    available = wait_for_deployment('nginx-{}'.format(run_id), url, token)

    if not available:
        fatal_error('Deployment failed')

    parts = urllib.parse.urlsplit(url)
    domain = parts.netloc.split('.', 1)[-1]
    available = wait_for_endpoint('http://nginx-{}-e2e.{}/'.format(run_id, domain), 'Welcome to nginx!', timeout=300)

    if not available:
        fatal_error('ELB service registration failed')
