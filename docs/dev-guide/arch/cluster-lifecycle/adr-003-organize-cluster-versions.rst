==============================================
ADR-003: Organize cluster versions in branches
==============================================

Context
=======

When managing multiple clusters with different SLOs there is a need for pinning
different clusters to different channels of the cluster configuration.  For
instance a production cluster might require a more stable channel of the
cluster configuration than a test or playground cluster where we want to try
out new, not yet stable, features.

To be able to manage multiple channels for different clusters we need to define
a process describing:

* What defines a channel.
* How to move patches/hotfixes between channels.
* How to promote an “unstable” channel to “stable”.
* How to try out experimental features.

Decision
========

Cluster configuration channels will map to git branches in the configuration
repository. The branch layout is shown below.

.. code-block:: text

    PR (experimental-branch-1)-
                               \
    PR (feature-2) ------------------> dev
                               /         \
    PR (hotfix-3) ----------------------> alpha
                               \             \
                                \----------> beta
                                              \
                                              stable

``dev`` is the default branch and is the main entrypoint for new feature PRs.
Every new feature should therefore start as a PR targeting ``dev`` and should
flow to the other channels only from the ``dev`` channel. Critical hotfixes can
go directly to the relevant channels.

Experimental features should be tested on a separate branch which is based on
```dev``` before they are merged into the ```dev``` branch.

Specifying the channel for a cluster is done by assigning a branch/channel name
to the channel field of a cluster resource in the Cluster Registry.

* TBD: when is something considered ready to be promoted? (after X days
  automatically)?
* TBD: how is something promoted from dev to alpha (and further up)?
* TBD: what controls do we need when promoting (four eyes?)?

Status
======

Proposed.

Consequences
============

* The default branch of kubernetes-on-aws becomes ``dev``.
* We need to protect ``dev``/``alpha``/``beta``/``stable`` branches.
