import urllib.parse
from clickclick import fatal_error

from .helpers import wait_for_deployment, create_deployment, create_service, wait_for_pod, create_resource, PETSET_PATH, SECRET_PATH


def test_volumes(run_id, url, token):


    secret_manifest = '''
apiVersion: v1
kind: Secret
metadata:
  name: &cluster_name spilodemo
  labels:
    application: spilo
    spilo-cluster: *cluster_name
type: Opaque
data:
  superuser-password: emFsYW5kbw==
  replication-password: cmVwLXBhc3M=
  admin-password: YWRtaW4=
'''

    create_resource(secret_manifest, url + SECRET_PATH, token)


    manifest = '''
apiVersion: apps/v1alpha1
kind: PetSet
metadata:
  name: &cluster_name spilodemo
  labels:
    application: spilo
    spilo-cluster: *cluster_name
spec:
  replicas: 3
  serviceName: *cluster_name
  template:
    metadata:
      labels:
        application: spilo
        spilo-cluster: *cluster_name
      annotations:
        pod.alpha.kubernetes.io/initialized: "true"
    spec:
      containers:
      - name: *cluster_name
        image: registry.opensource.zalan.do/acid/spilotest-9.6:1.1-p10  # put the spilo image here
        imagePullPolicy: Always
        ports:
        - containerPort: 8008
          protocol: TCP
        - containerPort: 5432
          protocol: TCP
        volumeMounts:
        - mountPath: /home/postgres/pgdata
          name: pgdata
        env:
        - name: ETCD_HOST
          value: 'etcd.default.svc.cluster.local:2379' # where is your etcd?
        - name: POD_IP
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.podIP
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        - name: PGPASSWORD_SUPERUSER
          valueFrom:
            secretKeyRef:
              name: *cluster_name
              key: superuser-password
        - name: PGPASSWORD_ADMIN
          valueFrom:
            secretKeyRef:
              name: *cluster_name
              key: admin-password
        - name: PGPASSWORD_STANDBY
          valueFrom:
            secretKeyRef:
              name: *cluster_name
              key: replication-password
        - name: SCOPE
          value: *cluster_name
        - name: PGROOT
          value: /home/postgres/pgdata/pgroot
      terminationGracePeriodSeconds: 0
      volumes:
      - name: pgdata
        emptyDir: {}
  volumeClaimTemplates:
  - metadata:
      labels:
        application: spilo
        spilo-cluster: *cluster_name
      annotations:
        volume.beta.kubernetes.io/storage-class: standard
      name: pgdata
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 5Gi
'''
    create_resource(manifest, url + PETSET_PATH, token)

    for i in range(3):
        available = wait_for_pod('spilodemo-{}'.format(i), url, token)
        if not available:
            fatal_error('e2e test for volumes failed')
