.. _logging:

=======
Logging
=======

Zalando cluster will ship logs to Scalyr for all containers running on a cluster node. The logs will include extra attributes/tags/metadata depending on deployment manifests. Whenever a new container starts on a cluster node, its logs will be shipped.

.. note::
    Logs are shipped per container and not per application. To view all logs from certain application you can use Scalyr UI https://www.scalyr.com/events and filter using :ref:`log-attributes-label`.

One Scalyr account will be provisioned for each community, i.e. the same Scalyr account is used for both test and production clusters.

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

In order for the container logs to be shipped, your deployment **must** include the following metadata labels:

- application
- version

.. _log-attributes-label:

Logs attributes
===============

All logs are shipped with extra attributes that can help in filtering from Scalyr UI (or API). Usually those extra fields are extracted from deployment labels, or the Kubernetes cluster/API.

``application``
    Application ID. Retrieved from metadata labels.

``version``
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

Log parsing
===========

The default parser for application logs is the ``json`` parser.
In some cases however you might want to use a `custom Scalyr parser
<https://www.scalyr.com/help/config>`_ for your application. This can be
achieved via Pod annotations.

However, the ``json`` parser only parses the JSON generated from the Docker logs. If your application
generates logs in JSON, the default parser will only see them as an escaped string of JSON.
However, Scalyr provides a special parser ``escapedJson`` for that.

Scalyr's default parser can even be configured to also make a pass with the ``escapedJson`` parser. That way
there is no need to configure anything on a per application level to get properly parsed fields from JSON based
application logs in Scalyr. Just `edit the JSON parser <https://www.scalyr.com/parsers?parser=json>`_ to contain the
following config.

.. code-block:: js

   // Parser for log files containing JSON records.
   {
      attributes: {
        // Tag all events parsed with this parser so we can easily select them in queries.
        dataset: "json"
      },

      formats: [
        {format: "${parse=json}$", repeat: true},
        {format: "\\{\"log\":\"$log{parse=escapedJson}$", repeat: true}
      ]
    }

The following example shows how to annotate a pod to instruct the log watcher
to use the custom parser ``json-java-parser`` for pod container ``my-app``.

.. code-block:: yaml

    apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: my-app
    spec:
      replicas: 3
      template:
        metadata:
          labels:
            application: my-app
          annotations:
            # specify scalyr log parser
            kubernetes-log-watcher/scalyr-parser: '[{"container": "my-app-container", "parser": "json-java-parser"}]'
        spec:
          containers:
          - name: my-app-container
            image: pierone.stups.zalan.do/myteam/my-app:cd53
            ports:
            - containerPort: 8080

The value of ``kubernetes-log-watcher/scalyr-parser`` annotation should be a
JSON serialized list. If ``container`` value did not match, then it will fall
back to the default parser (i.e. ``json``).

.. note::
    You need to specify the container in the parser annotation because
    you can have multiple containers in a pod which may use different log
    formats.
