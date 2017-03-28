=================
Kubernetes on AWS
=================

**WORK IN PROGRESS**

This repo contains configuration templates to provision Kubernetes_ clusters on AWS using Cloud Formation and `CoreOS Container Linux`_.

**Consider this as alpha quality**. Many values are hardcoded, and currently we're focusing on solving our own, specific/Zalando use case.
However, **we are open to ideas from the community at large about potentially turning this idea into a project that provides universal/general value to others**.
Please contact us via our Issues Tracker with your thoughts and suggestions.

Configuration in this repository initially was based on kube-aws_, but now depends on three components which are not yet open sourced:

* Cluster Registry to keep desired cluster states (e.g. used config channel and version)
* Cluster Lifecycle Manager to provision the cluster's Cloud Formation stack and apply Kubernetes manifests for system components
* Authnz Webhook to validate OAuth tokens and authorize access

We plan to release all required components to the community in Q1/2017.

Please see our `Running Kubernetes in Production on AWS document`_ for details on the setup.


Features
========

* Highly available master nodes (ASG) behind ELB
* Worker Auto Scaling Group
* Flannel overlay networking
* Cluster autoscaling (using kube-aws-autoscaler_)
* Route53 DNS integration via Mate_
* AWS IAM integration via kube2iam_
* Standard components are installed: kube-dns, heapster, dashboard, node exporter, kube-state-metrics
* Webhook authentication and authorization (roles "ReadOnly", "PowerUser", "Administrator")
* Log shipping via Scalyr
* Mostly done: full Ingress support (`#169 <https://github.com/zalando-incubator/kubernetes-on-aws/issues/169>`_)
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


Directory Structure
===================

* cluster: Senza Cloud Formation files and userdata (cloud-init) for ContainerLinux nodes
* cluster/manifests: Kubernetes manifests for system components (will be applied by Cluster Lifecycle Manager)
* docs: extracts from internal Zalando documentation (https://kubernetes-on-aws.readthedocs.io/)


.. _Kubernetes: http://kubernetes.io
.. _CoreOS Container Linux: https://coreos.com/os/docs/latest
.. _kube-aws: https://github.com/coreos/coreos-kubernetes/tree/master/multi-node/aws
.. _Senza Cloud Formation tool: https://github.com/zalando-stups/senza
.. _OAuth Token Info: http://planb.readthedocs.io/en/latest/intro.html#token-info
.. _Mate: https://github.com/zalando-incubator/mate
.. _kube2iam: https://github.com/jtblin/kube2iam
.. _kube-aws-autoscaler: https://github.com/hjacobs/kube-aws-autoscaler
.. _Running Kubernetes in Production on AWS document: https://kubernetes-on-aws.readthedocs.io/en/latest/admin-guide/kubernetes-in-production.html
