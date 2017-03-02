.. _service-accounts:

================
Service accounts
================

In Kubernetes, service accounts are used to provide an identity for pods.
Pods that want to interact with the API server will authenticate with a particular service account. By default, applications will authenticate as the ``default`` service account in the namespace they are running in.
This means, for example, that an application running in the ``test`` namespace will use the default service account of the ``test`` namespace.


Access Control
===============

Applications are authorized to perform certain actions based on the service account selected.
We currently allow the following service accounts:

kube-system:default
    Used only for admin access.
default:default
    Gives ReadOnly access to the Kubernetes API.
*:operator
    Gives full access to the used namespace and read and write access to TPR resources in all namespaces.


How to create service accounts
==============================

Service accounts can be created for your namespace via pipelines (or via ``zkubectl`` in test clusters) by placing the respective YAML in the ``apply`` folder and executing it.
For example, to request ``operator`` access you will need to create the following service account:

.. code-block:: yaml

    apiVersion: v1
    kind: ServiceAccount
    imagePullSecrets:
    - name: pierone.stups.zalan.do  # required to pull images from private registry
    metadata:
      name: operator
      namespace: $YOUR_NAMESPACE

This can be used in an example deployment like in the following YAML:

.. code-block:: yaml

    apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: nginx
      namespace: acid
    spec:
      replicas: 1
      template:
        metadata:
          labels:
            app: nginx
        spec:
          containers:
          - name: nginx
            image: nginx
            ports:
            - containerPort: 80
          serviceAccountName: operator  #this is where your service account is specified
          hostNetwork: true

