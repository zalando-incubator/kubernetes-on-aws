========================================
Example application with IAM credentials
========================================

.. Note::

    This section describes the legacy way of getting OAuth credentials via Mint.
    Please read :ref:`zalando-iam-integration` for the recommended new approach.

This is a full example manifest of an application (``myapp``) which uses IAM
credentials distributed via a mint-bucket (``zalando-stups-mint-12345678910-eu-central-1``).

Here is an example of a policy that grants access to the specific folder in the Mint's S3 bucket:

.. code-block:: json

    {
      "Version": "2012-10-17",
      "Statement": [
        {
          "Resource": [
            "arn:aws:s3:::zalando-stups-mint-12345678910-eu-central-1/myapp/*"
          ],
          "Effect": "Allow",
          "Action": [
            "s3:GetObject"
          ],
          "Sid": "AllowMintRead"
        }
      ]
    }

In this example the AWS access role for the S3 bucket is called ``myapp-iam-role``
(See also :doc:`iam-roles` for how to correctly setup such a role in AWS):

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
              image: myapp:v1.0.0
              env:
                - name: CREDENTIALS_DIR
                  value: /meta/credentials
              volumeMounts:
                - name: credentials
                  mountPath: /meta/credentials
                  readOnly: true
            - name: gerry
              image: registry.opensource.zalan.do/teapot/gerry:v0.0.14
              args:
                - /meta/credentials
                - --application-id=myapp
                - --mint-bucket=s3://zalando-stups-mint-12345678910-eu-central-1
              volumeMounts:
                - name: credentials
                  mountPath: /meta/credentials
                  readOnly: false
          volumes:
            - name: credentials
              emptyDir:
                medium: Memory # share a tmpfs between the two containers

The first important part of the manifest is the ``annotations`` section:

.. code-block:: yaml

  annotations:
    iam.amazonaws.com/role: myapp-iam-role

Here we specify the role needed in order for the pod to get access to the S3
bucket with the credentials.

The next important part is the ``gerry`` *sidecar*.

.. code-block:: yaml

    - name: gerry
      image: registry.opensource.zalan.do/teapot/gerry:v0.0.14
      args:
        - /meta/credentials
        - --application-id=myapp
        - --mint-bucket=s3://zalando-stups-mint-12345678910-eu-central-1
      volumeMounts:
        - name: credentials
          mountPath: /meta/credentials
          readOnly: false

The ``gerry`` *sidecar* container mounts the shared ``credentials`` mount point
under ``/meta/credentials`` and writes the credential files ``user.json`` and
``client.json`` to this location.

To read these files from the ``myapp`` container, the shared ``credentials``
mount point is also mounted into the ``myapp`` container.

.. code-block:: yaml

    - name: myapp
      image: myapp:v1.0.0
      env:
        - name: CREDENTIALS_DIR
          value: /meta/credentials
      volumeMounts:
        - name: credentials
          mountPath: /meta/credentials
          readOnly: true
