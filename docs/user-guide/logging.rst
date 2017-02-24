.. _logging:

=======
Logging
=======

Zalando cluster will ship logs to Scalyr for all containers running on a cluster node. The logs will include extra attributes/tags/metadata depending on deployment manifests. Whenever a new container starts on a cluster node, its logs will be shipped.

.. note::
    Logs are shipped per container and not per application. To view all logs from certain application you can use Scalyr UI https://www.scalyr.com/events and filter using :ref:`log-attributes-label`.

You need to make sure the minimum requirements are satisfied to start viewing logs on Scalyr.

Requirements
============

Logging output
--------------

Always make sure your application logs to ``stdout`` & ``stderr``. This will allow cluster log shipper to follow application logs, and also allows you to follow logs via Kubernetes native ``logs`` command.

.. code-block:: bash

    $ zkubectl logs -f my-pod-name my-container-name

Labels
------

In order for the container logs to be shipped, your deployment **must** include the follwoing metadata labels:

- application
- version

.. _log-attributes-label:

Logs attributes
===============

All logs are shipped with extra attributes that can help in filtering from Scalyr UI (or API). Usually those extra fields are extracted from deployment labels, or the Kubernetes cluster/API.

``application``
    Application ID. Retrieved from metadata labels.

``versions``
    Application version. Retrieved from metadata labels.

``release``
    Application release. Retrieved from metadata labels. *[optional]*

``cluster``
    Cluster ID. Retrieved from Kubernetes cluster.

``container``
    Container name. Retrieved from Kubernetes API.

``node``
    Cluster node running this container. Retrieved from Kubernetes cluster.

``pod``
    Pod name running the container. Retrieved from Kubernetes cluster.

``namespace``
    Namespace running this deployment(pod). Retrieved from Kubernetes cluster.

