=======================
TLS Termination and DNS
=======================

This section describes how to expose a service via TLS to the internet.

Expose your app
===============

Let's deploy a simple web server to test that our TLS termination works.

Submit the following ``yaml`` files to your cluster. Note that this guide uses a
top-down approach and starts with deploying the service first. You can, however,
submit the files in any order you like.

Create a service
----------------

Then create a ``Service`` of type ``LoadBalancer`` so that your pods become
accessible from the internet through an ``ELB``. For TLS termination to work
you need to annotate the service with the ARN of the certificate you want to serve.

.. code-block:: yaml

    apiVersion: v1
    kind: Service
    metadata:
      name: nginx
      annotations:
        service.beta.kubernetes.io/aws-load-balancer-ssl-cert: arn:aws:acm:eu-central-1:some-account-id:certificate/some-cert-id
        service.beta.kubernetes.io/aws-load-balancer-backend-protocol: http
    spec:
      type: LoadBalancer
      ports:
      - port: 443
        targetPort: 80
      selector:
        app: nginx

This creates a logical service called ``nginx`` that forwards all traffic to any pods
that match the label selector ``app=nginx``, which we haven't created yet. The service (logically) listens on port 443 and forwards to
port 80 on each of the upstream pods, which is where the nginx processes will listen on.

We also define the protocol that our upstreams use. Often your upstreams will just speak
plain HTTP so the second annotation's value is actually the default value and can be omitted.

**Make sure to define your service to listen on port 443 as this will be used as the listening
port for your ELB.**

Wait for a couple of minutes for AWS to provision an ``ELB`` for you and for DNS to propate.
Check the list of services to find out the endpoint of the ``ELB`` that was created for you.

.. code-block:: bash

    $ kubectl get svc -o wide
    NAME      CLUSTER-IP   EXTERNAL-IP                                     PORT(S)   AGE       SELECTOR
    nginx     10.3.0.245   some-long-hash.eu-central-1.elb.amazonaws.com   443/TCP   6m        app=nginx

Create the deployment
---------------------

Now let's deploy some pods that actually implement our service.

.. code-block:: yaml

    apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: nginx
    spec:
      replicas: 2
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

This creates a deployment called ``nginx`` that will ensure to run two copies
of the nginx image from dockerhub listening on port 80. They match exactly the
labels that our service is looking for so they be dynamically added to the
service's pool of upstreams.

Make sure your pods are running.

.. code-block:: bash

    $ kubectl get pods
    NAME                     READY     STATUS    RESTARTS   AGE
    nginx-1447934386-iblb3   1/1       Running   0          7m
    nginx-1447934386-jj559   1/1       Running   0          7m

Now ``curl`` the service endpoint. You'll get a certificate warning since the hostname
doesn't match the served certificate.

.. code-block:: bash

    $ curl --insecure https://some-long-hash.eu-central-1.elb.amazonaws.com
    <!DOCTYPE html>
    <html>
    <head>
    <title>Welcome to nginx!</title>
    ...
    </body>
    </html>


DNS records
===========

For convenience we create alias DNS entries for your service so you don't have
to use the arbitrary ELB endpoints. The DNS name that is generated is based on
your service's name, namespace, and the cluster's domain.

The generated DNS name's format is:

.. code-block:: go

    {serviceName}-{serviceNamespace}.{teamName}.zalan.do

For the service above this results in the following DNS name:

.. code-block:: go

    nginx-default.hackweek.zalan.do

Verify that this works with ``curl``. If you've chosen the right certificate ARN
you won't get any certificate warning.

.. code-block:: bash

    $ curl https://nginx-default.hackweek.zalan.do
    <!DOCTYPE html>
    <html>
    <head>
    <title>Welcome to nginx!</title>
    ...
    </body>
    </html>

Customizing the DNS name
------------------------

However, if you're not happy with the default DNS name you can change it by
adding an additional annotation to your service containing the desired dns name.

.. code-block:: yaml

    apiVersion: v1
    kind: Service
    metadata:
      name: nginx
      annotations:
        k8s.zalando.org/dnsname: my-nginx.hackweek.zalan.do
    spec:
      ...

Note that although you specify the full dns name here you must pick a name that
is inside the zone of the cluster, e.g. in this case ``*.hackweek.zalan.do``.
Also keep in mind that when doing this you can clash with other user's service names.


Make sure it works:

.. code-block:: bash

    $ curl https://my-nginx.hackweek.zalan.do
    <!DOCTYPE html>
    <html>
    <head>
    <title>Welcome to nginx!</title>
    ...
    </body>
    </html>

For reference, the full service description should look like this:

.. code-block:: yaml

    apiVersion: v1
    kind: Service
    metadata:
      name: nginx
      annotations:
        service.beta.kubernetes.io/aws-load-balancer-ssl-cert: arn:aws:acm:eu-central-1:some-account-id:certificate/some-cert-id
        service.beta.kubernetes.io/aws-load-balancer-backend-protocol: http
        k8s.zalando.org/dnsname: my-nginx.hackweek.zalan.do
    spec:
      type: LoadBalancer
      ports:
      - port: 443
        targetPort: 80
      selector:
        app: nginx

*Special note to Zalando Hackweek cluster users: We will provide you with the correct
ARN for the Hackweek cluster DNS zone.*

Common pitfalls
===============

Kubernetes clusters usually run an internal DNS server that allows you to reference services
from inside the cluster via DNS names rather than IPs. The internal DNS name for this example
is ``nginx.default.svc.cluster.local``. So, from inside any pod of the cluster you can lookup
your service with:

.. code-block:: bash

    dig +short nginx.default.svc.cluster.local
    10.3.0.245

But don't get confused due to the mixed ports: Your service just forwards to the plain
HTTP endpoints of your nginxs but serves them on port 443, as HTTP. So to avoid confusion
when accessing your service from another pod make sure to specify both port and protocol.

.. code-block:: bash

    $ curl http://nginx.default.svc.cluster.local:443
    <!DOCTYPE html>
    <html>
    <head>
    <title>Welcome to nginx!</title>
    ...
    </body>
    </html>

Note that we use HTTP on port 443 here.
