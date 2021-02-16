.. _aws-iam:

===================
AWS IAM integration
===================

This section describes how to setup an AWS IAM role which can then be assumed
by pods running in a Kubernetes cluster.
You only need AWS IAM roles if your application calls the AWS API directly (e.g. to store data in some S3 bucket).

Create IAM Role with AssumeRole trust relationship
==================================================

In order for an AWS IAM role to be assumed by the worker node and passed on
to a pod running on the node, it must allow the worker node IAM role to assume
it.

This is achieved by adding a trust relation on the role trust relationship
policy document. Assuming the account number is ``12345678912`` and the cluster
name is ``kube-1``, the policy document would look like this:

.. code-block:: json

    {
      "Version": "2012-10-17",
      "Statement": [
        {
          "Effect": "Allow",
          "Principal": {
            "AWS": "arn:aws:iam::12345678912:role/kube-1-worker"
          },
          "Action": "sts:AssumeRole"
        }
      ]
    }

Reference IAM role in pod
=========================

In order to use the IAM role in a pod you simply need to reference the role
name in an annotation on the pod specification. As an example we can create a
simple deployment for an application called ``myapp`` which require the IAM
role ``myapp-iam-role``:

.. code-block:: yaml

    apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: myapp
    spec:
      replicas: 1
      template:
        metadata:
          labels:
            app: myapp
          annotations:
            iam.amazonaws.com/role: myapp-iam-role
        spec:
          containers:
          - name: myapp
            image: myapp:v1.0

To test that the pod gets the correct role you can exec into the container and
query the metadata endpoint.

.. code-block:: none

    $ zkubectl exec -it myapp-podid -- sh
    $ curl -s 169.254.169.254/latest/meta-data/iam/security-credentials/
    myapp-iam-role

The response should be the name of the role available from
within the pod.
