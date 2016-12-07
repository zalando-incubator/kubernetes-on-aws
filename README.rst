=================
Kubernetes on AWS
=================

**WORK IN PROGRESS**

This repo contains some scripts and templates to provision Kubernetes clusters on AWS using Cloud Formation and CoreOS.

**Consider this as very early alpha quality**. Many values are hardcoded, and currently we're focusing on solving our own, specific/Zalando user case. However, **we are open to ideas from the community at large about potentially turning this idea into a project that provides universal/general value to others**. Please contact us via our Issues Tracker with your thoughts and suggestions.

It was initially based on `kube-aws`_, but we decided to diverge from it:

* kube-aws uses a single master EC2 instance --- we want to have an ASG for the master nodes (probably running with size 1 usually, but having the option for more, e.g. during updates/migrations etc)
* kube-aws runs etcd on the master --- we want to run etcd separately (currently we use our own 3 node etcd appliance with DNS discovery (SRV records))
* kube-aws does not configure an ELB for the API server --- we want to terminate SSL at ELB in order to leverage existing SSL infrastructure (including ACM)
* kube-aws uses a single CF template --- we want to split into at least 3 CF templates to facilitate future upgrades and extra node pools (one for etcd cluster, one for master and one for worker nodes)

We therefore adapted the generated Cloud Formation to YAML and are using our own `Senza Cloud Formation tool`_ for deployment (it's not doing any magic, but e.g. makes ELB+DNS config easy).

Features
========

* Highly available master nodes (ASG) behind ELB
* Worker Auto Scaling Group with Kubelet ELB health check
* Cluster autoscaling (using autoscaler from contrib repo)
* Route53 DNS integration via Mate_
* AWS IAM integration via kube2iam_
* Standard components are installed: heapster, dashboard, node exporter, kube-state-metrics
* Webhook authentication and authorization (roles "ReadOnly", "PowerUser", "Administrator")
* Planned: full Ingress support (`#169 <https://github.com/zalando-incubator/kubernetes-on-aws/issues/169>`_)
* Planned: Spot Fleet integration (`#61 <https://github.com/zalando-incubator/kubernetes-on-aws/issues/61>`_)

Notes
=====

* Node and user authentication is done via tokens (using the webhook feature)
* SSL client-cert authentication is disabled
* Many values are hardcoded
* Secrets (e.g. shared token) are not KMS-encrypted


Assumptions
===========

* The AWS account has a single Route53 hosted zone including a proper SSL cert (you can use the free ACM service)
* The VPC has at least one public subnet per AZ (either AWS default VPC setup or public subnet named "dmz-<REGION>-<AZ>")
* The VPC is in region eu-central-1 or eu-west-1
* etcd cluster is available via DNS discovery (SRV records) at etcd.<YOUR-HOSTED-ZONE>
* `OAuth Token Info`_ is available to validate user tokens


Usage
=====

.. code-block:: bash

    $ sudo pip3 install -U stups-senza awscli # install Senza and AWS CLI
    $ # login to AWS with right region
    $ cd cluster
    $ ./cluster.py create <STACK_NAME> <VERSION> # e.g. ./cluster.py create kube-aws-test 1

This will bootstrap a new cluster and make the API server available as https://<STACK_NAME>-<VERSION>.<YOUR-HOSTED-ZONE-DOMAIN>.

The authorization webhook will require the user to have the group "aws:<ACCOUNT_ID>:<REGION>:<STACK_NAME>:<VERSION>", e.g. "aws:123456789012:eu-central-1:kube-aws-test-1".

Update
======

Clusters can be updated with

.. code-block:: bash

    $ ./cluster.py update <STACK_NAME> <VERSION> # e.g. ./cluster.py update kube-aws-test 1

This will apply the new cloud-configs via cloud formation and trigger a rolling update for both workers and masters.

Instance Type
=============

Worker instance type can be configured on create and update.

On creation:

* provide optional flag ``--instance-type`` to specify instance type of worker nodes (defaults to ``t2.micro``)

On update:

* provide optional flag ``--instance-type´` to change instance type of worker nodes (defaults to ``current`` which results in whatever type the workers currently have)
* if cloud-config didn't change one has to use the ``--force`` flag to trigger the update

Testing
=======

You can run end-to-end tests against a running cluster:

.. code-block:: bash

    $ cd e2e
    $ sudo pip3 install -r requirements.txt
    $ ./test-cluster.py <API_SERVER_URL> --token=<API_TOKEN>

Where ``API_SERVER_URL`` is your cluster's API endpoint (e.g. https://kube-1.myteam.example.org) and ``API_TOKEN`` is a valid Bearer token.
You can use ``./cluster.py get-api-token <STACK_NAME> <VERSION>`` to get the worker's shared secret from the AWS user data.


.. _kube-aws: https://github.com/coreos/coreos-kubernetes/tree/master/multi-node/aws
.. _Senza Cloud Formation tool: https://github.com/zalando-stups/senza
.. _OAuth Token Info: http://planb.readthedocs.io/en/latest/intro.html#token-info
.. _Mate: https://github.com/zalando-incubator/mate
.. _kube2iam: https://github.com/jtblin/kube2iam
