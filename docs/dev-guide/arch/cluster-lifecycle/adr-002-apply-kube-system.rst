================================================================
ADR-002: Installation of Kubernetes non core system components
================================================================


Context
=======

In ``cluster.py`` we used to install all the ``kube-system`` components using a ``systemd`` unit. This consisted basically in a bash script that deployed all the manifests from ``/srv/kubernetes/manifests/*/*.yaml`` using ``kubectl``.
We obviously do not want to update versions manually via kubectl. Furthermore, this approach also meant that we had to launch a new master instance in order to apply the updated manifests.

Decision
========

We will do the following:

- remove entirely the "install-kube-system" unit from the master user data.
- create a folder with all the manifests for each of the kubernetes artifact
- apply all the manifests from the Cluster Lifecycle Manager code

Some of the possible alternatives for the folder structures are:

1. /manifests/APPLICATION_NAME/deployment.yaml - which uses a folder structure that includes the APPLICATION_NAME

2. /manifests/APPLICATION_NAME/KIND/mate.yaml - which uses a folder structure that includes APPLICATION_NAME and KIND

3. /manifests/mate-deployment.yaml - where we have a flat structure and the filenames contain the name of the application and the kind

4. /manifests/mate.yaml - where mate.yaml contains all the artifacts of all kinds related to mate

We choose number 1 as it seems the most compelling alternative.
Number 2 will only introduce an additional folder level that does not provide any benefit. Number 3 will instead rely on a naming convention on the given kind.
Number 4, instead, is a competitive alternative to number 1 and could be adopted, but we prefer to go with number 1 as this is very flexible and probably more readable for the maintainer.
For the file naming convention, we recommend to split in files for kind when is possible and put the name (or just a prefix) in the file name. We will not make any assumption on the file naming scheme in the code.
Also, no assumption will be made on the order of execution of such files.

Status
======

Accepted.

Consequences
============

The chosen file convention will be relevant when discussing the removal of components from ``kube-system``.
This is currently out of scope for this ADR as this only covers the "apply" case.
