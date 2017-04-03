.. _faq:

===
FAQ
===

How do I...
-----------

... ensure that my application runs in multiple Availability Zones?
    The Kubernetes scheduler will automatically try to distribute application replicas (pods) across multiple "failure domains" (the Kubernetes term for AZs).

... use the AWS API from my application on Kubernetes?
    Create an IAM role via Cloud Formation and assign it to your application pods.
    The AWS SDKs will automatically use the assigned IAM role. See :ref:`aws-iam` for details.

... get OAuth access tokens in my application on Kubernetes?
    Your application can declare needed OAuth credentials (tokens and clients) via the ``PlatformCredentialsSet``. See :ref:`zalando-iam-integration` for details.

... read the logs of my application?
    The most convenient way to read your application's logs (stdout and stderr) is by filtering by the ``application`` label in the Scalyr UI. See :ref:`logging` for details.

... switch traffic gradually to a new application version?
    Traffic switching is currently implemented by scaling up the new deployment and scaling down the old version.
    This process is fully automated and cannot be controlled in the current CI/CD Jenkins pipeline.
    The future deployment infrastructure will probably support manual traffic switching.

... use different namespaces?
    We recommend using the "default" namespace, but you can create your own if you want to.
