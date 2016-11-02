from clickclick import fatal_error

from .helpers import wait_for_deployment, create_deployment


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

    available = wait_for_deployment('nginx-{}'.format(run_id), url, token)

    if not available:
        fatal_error('Deployment failed')
