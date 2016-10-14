=================
Kubernetes on AWS
=================

**WORK IN PROGRESS**

This repo contains some scripts to provision Kubernetes clusters on AWS using Cloud Formation and CoreOS.

**Consider this very early test stuff, many values are hardcoded and we are only trying to solve or own specific Zalando user case!**

It was initially based on `kube-aws`_, but we decided to diverge from it:

* kube-aws deploys a new VPC --- we want to deploy into an existing public subnet
* kube-aws uses a single master EC2 instance --- we want to have an ASG for the master nodes (probably running with size 1 usually, but having the option for more, e.g. during updates/migrations etc)
* kube-aws runs etcd on the master --- we want to run etcd separately (currently we use our own 3 node etcd appliance with DNS discovery (SRV records))
* kube-aws does not configure an ELB for the API server --- we want to terminate SSL at ELB in order to leverage existing SSL infrastructure (including ACM)
* kube-aws uses a single CF template --- we want to split into at least 3 CF templates to facilitate future upgrades and extra node pools (one for etcd cluster, one for master and one for worker nodes)

We therefore adapted the generated Cloud Formation to YAML and are using our own `Senza Cloud Formation tool`_ for deployment (it's not doing any magic, but e.g. makes ELB+DNS config easy).

Assumptions
===========

* The AWS account has a single Route53 hosted zone including a proper SSL cert (you can use the free ACM service)
* The VPC has public subnets called "dmz-*-a"
* The VPC is in region eu-central-1 or eu-west-1
* etcd cluster is available via DNS discovery (SRV records) at etcd.<YOUR-HOSTED-ZONE>
* OAuth Token Info is available to validate user tokens


Usage
=====

.. code-block:: bash

    $ sudo pip3 install -U stups-senza awscli # install Senza and AWS CLI
    $ # login to AWS with right region
    $ cd cluster
    $ ./create-stack.sh <AZ-SUFFIX> <VERSION> # e.g. ./create-stack.sh a test1

This will bootstrap a new cluster and make the API server available as https://kube-aws-test-<VERSION>.<YOUR-HOSTED-ZONE-DOMAIN>.


.. _kube-aws: https://github.com/coreos/coreos-kubernetes/tree/master/multi-node/aws
.. _Senza Cloud Formation tool: https://github.com/zalando-stups/senza
