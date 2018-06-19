# Contributing Guide

*FAQ*

## What is the branch for active development?

Active development happens in the "dev" branch. If you have a
contribution just create a PR to it.

## How do I run tests?

Once you add the
["ready-to-test"](https://github.com/zalando-incubator/kubernetes-on-aws/labels/ready-to-test)
label, tests will automatically be kicked off. Our internal [Jenkins
instance](https://teapot.ci.zalan.do/) is used to run the tests. The CI
config for the project can be found at
[teapot/kubernetes-on-aws-e2e](https://github.bus.zalan.do/teapot/kubernetes-on-aws-e2e)
on our internal Github Enterprise.

## How do I roll out changes?

We only support rolling out changes to our internal clusters so far. Our
Kubernetes clusters are divided into multiple channels. Each channel has
a corresponding branch. For example, the ["alpha"
branch](https://github.com/zalando-incubator/kubernetes-on-aws/tree/alpha)
represents the state of clusters in the "alpha" channel. See [Cluster
Provisioning](https://kubernetes-on-aws.readthedocs.io/en/latest/admin-guide/kubernetes-in-production.html#cluster-provisioning) to learn more. To roll out changes to a particular
channel, create a PR to it's corresponding branch e.g
[dev-to-alpha](https://github.com/zalando-incubator/kubernetes-on-aws/pull/1130).
You may use the [prepare-pr.sh](https://github.com/zalando-incubator/kubernetes-on-aws/blob/dev/prepare-pr.sh) script to create a PR.