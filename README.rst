=================
Kubernetes on AWS
=================

**WORK IN PROGRESS**

This repo contains configuration templates to provision Kubernetes_ clusters on AWS using `Cloud Formation`_ and `Ubuntu Linux`_.

Many values are parameterized and values are not always visible. We're focusing on solving our own, specific/Zalando use case.
However, **we are open to ideas from the community at large about potentially turning this idea into a project that provides universal/general value to others**.
Please contact us via our Issues Tracker with your thoughts and suggestions.

Configuration in this repository initially was based on kube-aws_, but now depends on four components which aren't all yet open sourced:

* Cluster Registry to keep desired cluster states (e.g. used config channel and version)
* `Cluster Lifecycle Manager`_ to provision the cluster's Cloud Formation stack and apply Kubernetes manifests for system components
* Cluster Lifecycle Controller that handles rolling updates from inside the cluster, for example node termination
* Authnz Webhook to validate OAuth tokens and authorize access

Lean more about Zalando's cloud native journey by reading the `Zalando Case Study on kubernetes.io`_.
Please watch our meetup talk `"Kubernetes on AWS at Europe's Leading Online Fashion Platform" on YouTube`_ to learn how we run Kubernetes on AWS in production.
See our `Running Kubernetes in Production on AWS document`_ for details on the setup.


Features
========

* Highly available master nodes (ASG) behind ELB
* Worker Auto Scaling Group with node pools support
* Flannel overlay networking
* Cluster autoscaling (using cluster-autoscaler_)
* Kubernetes DNS with node-local dnsmasq as daemonset and CoreDNS resolver for cluster.local domain running in the same pod.
* Route53 DNS integration via `External DNS`_
* AWS IAM integration via kube2iam_, `AWS OIDC IAM`_
* Standard components are installed: dashboard, node exporter, kube-state-metrics, see also `cluster/manifests`_ directory
* Webhook authentication and authorization (roles "ReadOnly", "PowerUser", "Manual", "Emergency", "Administrator")
* Emergency Access via internal emergency-access-service, that grant roles "Manual" and "Emergency" with 4 eyes principle and audit logging
* Log shipping via Scalyr
* Full Ingress support with ALB/NLB and TLS integration via kube-ingress-aws-controller_ and HTTP routing via skipper_
* Enhanced usability with managed stacks and blue green deployments via stackset-controller_ and skipper_
* `Fabric API Gateway`_, which can be used in combination with stackset-controller_
* Static Egress IPs to route through NAT Gateways with Elastic IPs via kube-static-egress-controller_
* Horizontal Pod Autoscaling with scaling by request per second, SQS queue size or others via kube-metrics-adapter_
* Vertical Pod Autoscaling to scale for example Prometheus
* EFS support
* GPU support
* ETCD backup via Kubernetes cronjob and etcdctl snapshot and upload to S3
* Monitoring via Prometheus and OpenTracing_
* Fully automated cluster updates via `Cluster Lifecycle Manager`_
* Automated downscaling for test clusters with kube-downscaler_
* Fallback node pools
* Spot node pool integration
* automated PDB creation with pdb-controller_


Notes
=====

* Node and user authentication is done via tokens (using the webhook feature)
* SSL client-cert authentication is disabled
* Many values are hardcoded
* Secrets (e.g. shared token) are not KMS-encrypted in the cluster


Assumptions
===========

* The AWS account has one or more hosted zones in Route53 including a proper SSL cert (you can use the free ACM service)
* The VPC has at least one public subnet per AZ (either AWS default VPC setup or public subnet named "dmz-<REGION>-<AZ>")
* The VPC is in region eu-central-1 or eu-west-1
* etcd cluster is available via DNS discovery (SRV records) at etcd.<YOUR-HOSTED-ZONE>
* `OAuth Token Info`_ is available to validate user tokens


Directory Structure
===================

* cluster/cluster.yaml: Cloud Formation template files for the cluster (will be applied by `Cluster Lifecycle Manager`_)
* cluster/config-defaults.yaml: Default values for different kind of use that can be overriden by values from our cluster-registry (will be applied by `Cluster Lifecycle Manager`_)
* cluster/etcd-cluster.yaml: Senza Cloud Formation to deploy ETCD
* cluster/manifests: Kubernetes manifests for system components (will be applied by `Cluster Lifecycle Manager`_)
* cluster/node-pools: Cloud Formation template files and userdata (cloud-init) for ContainerLinux node-pools (will be applied by `Cluster Lifecycle Manager`_)
* docs: extracts from internal Zalando documentation (https://kubernetes-on-aws.readthedocs.io/)


.. _Kubernetes: http://kubernetes.io
.. _Cloud Formation: https://aws.amazon.com/cloudformation/
.. _Ubuntu Linux: https://ubuntu.com/
.. _CoreOS Container Linux: https://coreos.com/os/docs/latest
.. _kube-aws: https://github.com/kubernetes-retired/kube-aws
.. _Senza Cloud Formation tool: https://github.com/zalando-stups/senza
.. _OAuth Token Info: http://planb.readthedocs.io/en/latest/intro.html#token-info
.. _Cluster Lifecycle Manager: https://github.com/zalando-incubator/cluster-lifecycle-manager
.. _External DNS: https://github.com/kubernetes-incubator/external-dns
.. _kube2iam: https://github.com/jtblin/kube2iam
.. _kube-aws-iam-controller: https://github.com/zalando-incubator/kube-aws-iam-controller
.. _AWS OIDC IAM: https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
.. _cluster-autoscaler: https://github.com/kubernetes/autoscaler
.. _Running Kubernetes in Production on AWS document: https://kubernetes-on-aws.readthedocs.io/en/latest/admin-guide/kubernetes-in-production.html
.. _"Kubernetes on AWS at Europe's Leading Online Fashion Platform" on YouTube: https://www.youtube.com/watch?time_continue=2671&v=XmnhzEoengI
.. _kube-ingress-aws-controller: https://github.com/zalando-incubator/kube-ingress-aws-controller
.. _skipper: https://github.com/zalando/skipper
.. _stackset-controller: https://github.com/zalando-incubator/stackset-controller
.. _Fabric API Gateway: https://github.com/zalando-incubator/fabric-gateway
.. _kube-static-egress-controller: https://github.com/szuecs/kube-static-egress-controller
.. _kube-metrics-adapter: https://github.com/zalando-incubator/kube-metrics-adapter
.. _Zalando Case Study on kubernetes.io: https://kubernetes.io/case-studies/zalando/
.. _cluster/manifests: https://github.com/zalando-incubator/kubernetes-on-aws/tree/dev/cluster/manifests
.. _kube-downscaler: https://github.com/hjacobs/kube-downscaler
.. _pdb-controller: https://github.com/mikkeloscar/pdb-controller
.. _OpenTracing: https://opentracing.io
