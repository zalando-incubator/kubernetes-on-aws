=========================
Container resource limits
=========================

.. Note::

   This is a preliminary summary from skimming docs and educational guessing.
   No evaluation done. It could contain errors.

Resource definitions
====================

There are two supported resource types: ``cpu`` and ``memory``. In future versions of Kubernetes
one will be able to add custom resource types and the current implementation might be
based on that.

CPU resources are measured in virtual cores or more commonly in "millicores" (e.g. ``500m`` denoting 50% of a vCPU).
Memory resources are measured in Bytes and the usual suffixes can be used, e.g. ``500Mi`` denoting 500 Mebibyte_.

For each resource type there are two kinds of definitions: ``requests`` and ``limits``.
Requests and limits are defined per container. Since the unit of scheduling is a pod
one needs to sum them up to get the requests and limits of a pod.

The resulting four combinations are explained in more detail below.

Resource requests
-----------------

In general, requests are used by the scheduler to find a node that has free resources
to take the pod. A node is full when the sum of all requests equals the registered
capacity of that node in any resource type. So, if the requests of a pod are still
unclaimed on a node, the scheduler can schedule a pod there.

Note that this is the only metric the scheduler uses (in that context). It doesn't take
the actual usage of the pods into account (which can be lower or higher than whatever
is defined in requests).

Memory requests
    Used for finding nodes with enough memory and making better scheduling decisions.
CPU requests
    Maps to the docker flag ``--cpu-shares``, which defines a relative weight of that container
    for CPU time. The relative share is executed per core, which can lead to unexpected outcomes
    but probably nothing to worry about in our use cases. A container will never be killed
    because of this metric.

Resource limits
---------------

Limits define the upper bound of resources a container can use. Limits must always be greater
or equal to requests. The behavior differs between CPU and memory.

Memory limits
    Maps to the docker flag ``--memory``, which means processes in the container get killed by the
    kernel if they hit that memory usage (``OOMKilled``). Given you run one process per container this will kill
    the whole container and Kubernetes will try to restart it.
CPU limits
    Maps to the docker flag ``--cpu-quota``, which limits CPU time of that container's processes.
    Seems like you can define that a container can only max utilize a core by e.g. 50%.
    But, let's assume you have 3 of them on a single-core node this can lead to over-utilizing it.

Conclusion
==========

* ``requests`` are for making scheduling decisions
* ``limits`` are real resource limits of containers
* effect of CPU limits is not completely straight forward to understand
* choosing higher ``limits`` than ``requests`` allows to over-provision nodes,
  but has the danger of over-utilizing it
* ``requests`` are required for using the horizontal pod autoscaler

.. _Mebibyte: https://en.wikipedia.org/wiki/Mebibyte
