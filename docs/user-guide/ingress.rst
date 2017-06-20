.. _ingress:

=======
Ingress
=======

This section describes how to expose a service to the internet by defining Ingress rules.

What is Ingress?
================

Ingress allows to expose a service to the internet by defining its HTTP layer address. Ingress settings include:

* TLS certification
* host name
* path endpoint (optional)
* service and service port

The Ingress services, when detecting a new or modified Ingress entry, will create/update the DNS record for the defined
hostname, will update the load balancer to use a TLS certificate and route the requests to the cluster
nodes, and will define the routes that find the right service based on the hostname and the path.

More details about the general Ingress in Kubernetes can be found in the official `Ingress Resources`_.

How to setup Ingress?
=====================

Let's assume that we have a deployment with label ``application=test-app``, providing an API service on port
8080 and an admin UI on port 8081. In order to make them accessible from the internet, we need to create a
service first.

Create a service
----------------

The service definition looks like this, create it in the ``apply`` directory as ``service.yaml``:

.. code-block:: yaml

    apiVersion: v1
    kind: Service
    metadata:
      name: test-app-service
      labels:
        application: test-app-service
    spec:
      ports:
      - port: 8080
        protocol: TCP
        targetPort: 8080
        name: main-port
      - port: 8081
        protocol: TCP
        targetPort: 8081
        name: admin-ui-port
      selector:
        application: test-app

Note that we didn't define the ``type`` of the service. This means that the service type will be the default ``ClusterIP``, and
will be accessible only from inside the cluster.

Create the Ingress rules
------------------------

Let's assume that we want to access this API and admin UI from the internet with the base URL
https://test-app.playground.zalan.do, and we want to access the UI on the path ``/admin`` while all other endpoints
should be directed to the API. We can create the following Ingress entry in the ``apply`` directory as ``ingress.yaml``:

.. code-block:: yaml

    apiVersion: extensions/v1beta1
    kind: Ingress
    metadata:
      name: test-app
    spec:
      rules:
      - host: test-app.playground.zalan.do
        http:
          paths:
          - backend:
              serviceName: test-app-service
              servicePort: main-port
          - path: /admin
            backend:
              serviceName: test-app-service
              servicePort: admin-ui-port

Once the changes were applied by the pipeline, the API and the admin UI should be accessible at
https://test-app.playground.zalan.do and https://test-app.playground.zalan.do/admin. (If the load balancer and/or
the DNS entry are newly created, it can take ~1 minute for everything to
be ready.)
Already provisioned X509 Certificate (IAM and ACM) will be found and
matched automatically for your Ingress resource.

Manually selecting a certificate
--------------------------------

The right certificate is usually discovered automatically,
but there might be occasions where the SSL certificate ID (ARN) needs to be specified manually
(e.g. if a ``CNAME`` in another account points to our Ingress).
Let's assume we want to hard code our certificate that is used in the
ALB to terminate TLS for https://test-app.playground.zalan.do/.
We can create the following Ingress entry in the ``apply`` directory as ``ingress.yaml``:

.. code-block:: yaml

    apiVersion: extensions/v1beta1
    kind: Ingress
    metadata:
      name: test-app
      annotations:
        zalando.org/aws-load-balancer-ssl-cert: <certificate ARN>
    spec:
      rules:
      - host: test-app.playground.zalan.do
        http:
          paths:
          - backend:
              serviceName: test-app-service
              servicePort: main-port


Certificate ARN
---------------

In the above template, the token <certificate ARN> is meant to be replaced with the ARN of a valid certificate
available for your account. You can find the right certificate in one of the following two ways:

**1. For standard IAM certificates:**

.. code-block:: sh

    aws iam list-server-certificates

... should display something like this:

.. code-block:: json

    {
        "ServerCertificateMetadataList": [
            {
                "ServerCertificateId": "ABCDEFGHIJKLMNOPFAKE1",
                "ServerCertificateName": "self-signed-cert1",
                "Expiration": "2026-12-13T08:31:06Z",
                "Path": "/",
                "Arn": "arn:aws:iam::123456789012:server-certificate/self-signed-cert1",
                "UploadDate": "2016-12-15T08:48:03Z"
            },
            {
                "ServerCertificateId": "ABCDEFGHIJKLMNOPFAKE2",
                "ServerCertificateName": "self-signed-cert2",
                "Expiration": "2026-12-13T08:51:22Z",
                "Path": "/",
                "Arn": "arn:aws:iam::123456789012:server-certificate/self-signed-cert2",
                "UploadDate": "2016-12-15T08:51:41Z"
            },
            {
                "ServerCertificateId": "ABCDEFGHIJKLMNOPFAKE3",
                "ServerCertificateName": "teapot-zalan-do",
                "Expiration": "2023-05-11T00:00:00Z",
                "Path": "/",
                "Arn": "arn:aws:iam::123456789012:server-certificate/teapot-zalan-do",
                "UploadDate": "2016-05-12T12:26:52Z"
            }
        ]
    }

...where you want to use the ``Arn`` values.

**2. For Amazon Certificate Manager (ACM) certificates:**

.. code-block:: sh

    aws acm list-certificates

...should print something like this:

.. code-block:: json

    {
        "CertificateSummaryList": [
            {
                "CertificateArn": "arn:aws:acm:eu-central-1:123456789012:certificate/12345678-1234-1234-1234-123456789012",
                "DomainName": "teapot.zalan.do"
            },
            {
                "CertificateArn": "arn:aws:acm:eu-central-1:123456789012:certificate/12345678-1234-1234-1234-123456789012",
                "DomainName": "*.teapot.zalan.do"
            }
        ]
    }

...where you want to use the ``CertificateArn`` values.

Alternatives
============

You can expose an application with its own load balancer, described in the
:ref:`tls-termination`. The two methods can live next to each other, but they need to have separate
service definitions (due to the different service types).

.. _Ingress Resources: http://kubernetes.io/docs/user-guide/ingress/
