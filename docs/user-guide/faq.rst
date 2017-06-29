.. _faq:

===
FAQ
===

How do I...
-----------

... ensure that my application runs in multiple Availability Zones?
    The Kubernetes scheduler will automatically try to distribute pods across multiple "failure domains" (the Kubernetes term for AZs).

... use the AWS API from my application on Kubernetes?
    Create an IAM role via CloudFormation and assign it to your application pods.
    The AWS SDKs will automatically use the assigned IAM role. See :ref:`aws-iam` for details.

... get OAuth access tokens in my application on Kubernetes?
    Your application can declare needed OAuth credentials (tokens and clients) via the ``PlatformCredentialsSet``. See :ref:`zalando-iam-integration` for details.

... read the logs of my application?
    The most convenient way to read your application's logs (stdout and stderr) is by filtering by the ``application`` label in the Scalyr UI. See :ref:`logging` for details.

... get access to my Scalyr account?
    You can approach one of your colleagues who already has access to invite you to your Scalyr account.

... switch traffic gradually to a new application version?
    Traffic switching is currently implemented by scaling up the new deployment and scaling down the old version.
    This process is fully automated and cannot be controlled in the current CI/CD Jenkins pipeline.
    The future deployment infrastructure will probably support manual traffic switching.

... use different namespaces?
    We recommend using the "default" namespace, but you can create your own if you want to.

... quickly get write access to production clusters for 24x7 emergencies?
    We still need to set up the Emergency Operator workflow: the idea is to quickly give full access to production accounts and clusters in case of incidents. Eric’s idea is to require a real 24x7 INCIDENT ticket for getting access (this would ensure that it’s not misused for day-to-day work). Right now (2017-05-15) you can call STUPS 24x7 2nd level (via 1st level) to ask for emergency access.

... use a single Jenkins for both building (CI) and deployment (CD)? That would enable more sophisticated pipelines because no extra communication between CI and CD was needed. CI needs feedback look from CD in order to perform joint activities.
    The Jenkins setup will be replaced by the Continuous Delivery Platform which performs both builds and deploys. See https://pages.github.bus.zalan.do/continuous-delivery/cdp-docs/ and watch out for announcements and Friday Demos.

... test deployment YAMLs from CLI?
    ``$ zdeploy render-template deployment.yaml application=xxx version=xx | zkubectl create``

... access a production service from my test cluster?
    Test clusters are not allowed to get production OAuth credentials, please use a staging service and sandbox OAuth credentials.

... decide when to place a declaration under the ``apply`` folder, and when at the root (it doesn't seem to be standard)?
    The current Jenkins CI/CD pipeline relies on some Zalando conventions: every ``.yaml`` file in the ``apply`` folder is applied as a Kubernetes manifest or Cloud Formation template. Some files need to be on the "root" folder as they are processed in a special way, these files are e.g.: ``deployment.yaml``, ``autoscaling.yaml`` and ``pipeline.yaml``.

... use Helm_ together with Kubernetes on AWS?
    We don't currently (May 2017) support it because it requires the installation of some components in the ``kube-system`` namespace. This namespace is reserved for core cluster components as defined in the `Kubernetes on AWS configuration`_ and is not accessible to users.
    Furthermore, the Zalando "compliance by default" requirements (delivering stacks over declarations in a Zalando git repo) would clash with Helm defaults.


Will the cluster scale up automatically and quickly in case of surprise need of more pods?
------------------------------------------------------------------------------------------

Cluster autoscaling is purely based on resource requests, i.e. as soon as the resource requests increase (e.g. because the number of pods goes up) the autoscaler will set a new DesiredCapacity of the ASG. The autoscaler is very simple and not based on deltas, but on absolute numbers, i.e. it will potentially scale up by many nodes at once (not one by one). See https://github.com/hjacobs/kube-aws-autoscaler#how-it-works

.. _Helm: http://helm.sh
.. _Kubernetes on AWS configuration: https://github.com/zalando-incubator/kubernetes-on-aws
