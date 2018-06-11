==================
Contributing Guide
==================

*FAQ*

What is the branch for active development?
==========================================

Active development happens in the "dev" branch. If you have a contribution just
create a PR to it.

How do I run tests?
===================

Once you add the "ready-to-test", tests will automatically be kicked off.
Our internal `Jenkins instance`_ is used to run the tests. The CI config for the project can
be found at `teapot/kubernetes-on-aws-e2e`_ on our internal Github Enterprise.

How do I roll out changes?
==========================

We only support rolling out changes to our internal clusters so far. Our Kubernetes are divided into multiple channels.
Each channel has a corresponding branch. For example, the `"alpha" branch`_ represents the state of clusters in the
"alpha" channel. See `kubernetes-on-aws#cluster-provisioning`_ for details. To roll out changes to a particular channel,
creating a PR to it's corresponding branch e.g `dev-to-alpha`_. You may use the `prepare.sh` script to create a PR.



.. _Jenkins instance: https://teapot.ci.zalan.do/
.. _teapot/kubernetes-on-aws-e2e : https://github.bus.zalan.do/teapot/kubernetes-on-aws-e2e
.. _"alpha" branch: https://github.com/zalando-incubator/kubernetes-on-aws/tree/alpha
.. _"kubernetes-on-aws#cluster-provisioning": https://kubernetes-on-aws.readthedocs.io/en/latest/admin-guide/kubernetes-in-production.html#cluster-provisioning
.. _dev-to-alpha : https://github.com/zalando-incubator/kubernetes-on-aws/pull/1130
.. _`prepare.sh` : https://github.com/zalando-incubator/kubernetes-on-aws/blob/dev/prepare-pr.sh