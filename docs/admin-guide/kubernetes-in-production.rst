================================
Running Kubernetes in Production
================================

This document should briefly describe our leanings in Zalando Tech while running Kubernetes on AWS in production. As we just recently started to migrate to Kubernetes, we consider ourselves far from being experts in the field. This document is shared in the hope that others in the community can benefit from our learnings.

Context
=======

We are a team of infrastructure engineers provisioning Kubernetes clusters for our Zalando Tech delivery teams. We plan to have more than 30 production Kubernetes clusters. The following goals might help to understand the remainder of the document, our Kubernetes setup and our specific challenges:

* No manual operations: all cluster updates and operations need to be fully automated.
* No pet clusters: clusters should all look the same and not require any specific configurations/tweaking
* Reliability: the infrastructure should be rock-solid for our delivery teams to entrust our clusters with their most critical applications
* Autoscaling: clusters should automatically adapt to deployed workloads and hourly scaling events are expected
* Seamless migration: Dockerized twelve-factor apps currently deployed on AWS/STUPS should work without modifications on Kubernetes

Cluster Provisioning
====================

There are many tools out there to provision Kubernetes clusters. We chose to adapt kube-aws as it matches our current way of working on AWS: immutable nodes configured via cloud-init and Cloud Formation for declarative infrastructure. CoreOS’ Container Linux perfectly matches our understanding of the node OS: only provide what is needed to run containers, not more.

Only one Kubernetes cluster is created per AWS account. We create separated AWS accounts/clusters for production and test environments.

We always create two AWS Auto Scaling Groups (ASGs, “node pools”) right now:

* One master ASG with always two nodes which run the API server and controller-manager
* One worker ASG with 2 to N nodes to run application pods

Both ASGs span multiple Availability Zones (AZ). The API server is exposed with TLS via a “classic” TCP/SSL Elastic Load Balancer (ELB).

We use a custom built Cluster Registry REST service to manage our Kubernetes clusters. Another component (Cluster Lifecycle Manager, CLM) is regularly polling the Cluster Registry and updating clusters to the desired state. The desired state is expressed with Cloud Formation and Kubernetes manifests stored in git. Different clusters can use different channel configurations, i.e. some non-critical clusters might use the “alpha” channel with latest features while others rely on the “stable” channel. The channel concept is similar to how CoreOS manages releases of Container Linux.

*TODO: briefly describe our Cluster Lifecycle Manager and node update process*

AWS Integration
===============

We provision clusters on AWS and therefore want to integrate with AWS services where possible. The kube2iam daemon conveniently allows to assign an AWS IAM role to a pod by adding an annotation. Our infrastructure components such as the autoscaler use the same mechanism to access the AWS API with special (restricted) IAM roles.

Ingress
=======

There is no official way of implementing Ingress on AWS. We decided to create a new component Kube AWS Ingress Controller to achieve our goals:

* SSL termination by ALB: convenient usage of ACM (free Amazon CA) and certificates upload to AWS IAM
* Using the “new” ELBv2 Application Load Balancer

We use Skipper as our HTTP proxy to route based on Host header and path. Skipper directly comes with a Kubernetes data client to automatically update its routes periodically.

Mate is automatically configuring the Ingress hosts as DNS records in Route53 for us.

Resources
=========

Understanding the Kubernetes resource requests and limits is crucial.

Default resource requests and limits can be configured via the LimitRange resource. This can prevent “stupid” incidents like JVM deployments without any settings (no memory limit and no JVM heap set) eating all the node’s memory.

We provide a tiny script and use the Downwards API to conveniently run JVM applications on Kubernetes without the need to manually set the maximum heap size.

Kubelet can be instructed to reserve a certain amount of resources for the system and for Kubernetes components (kubelet itself and Docker etc). Reserved resources are subtracted from the node’s allocatable resources. This improves scheduling and makes resource allocation/usage more transparent. Node allocatable resources or rather reserved resources are also visible in Kubernetes Operational View.

TODO: add link to Node Allocatable design doc, kubelet flags and Kubernetes Operational View.

Graceful Pod Termination
========================

Kubernetes will cause service disruptions on pod terminations by default as applications and configuration need to be prepared for graceful shutdown. By default, pods receive the TERM signal and will be killed 30s later with KILL. Kubernetes expects the container to handle the TERM signal and change the readynessProbe to “fail”. This can be achieved by changing an in-memory value which instructs the health endpoint to return a non-200 status code. Sadly the default probe settings (period 10s, failure threshold 3) prevent an in-time change of the “ready” state. To achieve graceful pod shutdown..

* ..the container needs to switch its health endpoint to non-200 status code when receiving the TERM signal
* ..the readinessProbe settings need to be tweaked to react within the grace period (30s by default), e.g. by setting failure threshold to 1 or 2 (while keeping the default period of 10s)

TODO: example and link existing blog posts by others

Kubernetes’ assumption that application handle the TERM signal and change their health endpoint is a blocker for seamless migration from our AWS/STUPS infrastructure to Kubernetes. In STUPS, single Docker containers run directly on EC2 instances. Graceful container termination is not needed as AWS automatically deregisters EC2 instances and drains connections from the ELB on instance termination. We therefore consider solving the graceful pod termination issue in Kubernetes on the infrastructure level. This would not require any application code changes by our users (application developers).

Autoscaling
===========

Pod Autoscaling
---------------

We are using the HorizontalPodAutoscaler resource to scale the number of deployment replicas. Pod autoscaling requires implementing graceful pod termination (see above) to downscale safely in all circumstances. We only used the CPU-based pod autoscaling until now.

Node Autoscaling
----------------

Our experimental AWS Autoscaler is an attempt to implement a simple and elastic autoscaling with AWS Auto Scaling Groups.

Graceful node shutdown is required to allow safe downscaling at any time. We simply added a small systemd unit to run kubectl drain on shutdown.

Upscaling or node replacement poses the risk of race conditions between application pods and required system pods (DaemonSet). We have not yet figured out a good way of postponing application scheduling until the node is fully ready. The kubelet’s Ready condition is not enough as it does not ensure that all system pods such as kube-proxy and kube2iam are running. One idea is using taints during node initialization to prevent application pods to be scheduled until the node is fully ready.

Monitoring
==========

TODO: ZMON in the cluster, prometheus node exporter, kube-state-metrics, kube-ops-view

Jobs
====

TODO: Describe CronJob usage and job cleaner

Security
========

We authorize access to the API server via a proprietary webhook which verifies the OAuth Bearer access token and looks up user’s roles via another small REST services (backed historically by LDAP).

Access to etcd should be restricted as it holds all of Kubernetes’ cluster data thus allowing tampering when accessed directly.

We use flannel as our overlay network which requires etcd by default to configure its network ranges. There is experimental support for the flannel backend to be switched to the Kubernetes API server. This allows restricting etcd access to the master nodes.

Kubernetes allows to define PodSecurityPolicy resources to restrict the use of “privileged” containers and similar features which allow privilege escalation.



