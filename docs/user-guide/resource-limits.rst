=========================
Container resource limits
=========================

*Note: preliminary summary from skimming docs and educational guessing.*
*no evaluation done. could contain errors.*

resource definitions
====================

there are two supported resource types: ``cpu`` and ``mem``. in future versions of k8s
one will be able to add custom resource types and the current implementation might be
based on that.

for each resource type there are two kinds of definitions: ``requests`` and ``limits``.
requests and limits are defined per container. since the unit of scheduling is a pod
one needs to sum them up to get the requests and limits of a pod.

the resulting four combinations are explained in more detail below.

resource requests
-----------------

in general, requests are used by the scheduler to find a node that has free resources
to take the pod. a node is full when the sum of all requests equals the registered
capacity of that node in any resource type. so, if the requests of a pod are still
unclaimed on a node, the scheduler can schedule a pod there.

note, that this is the only metric the scheduler uses (in that context).it doesn't take
the actual usage of the pods into account (which can be lower or higher than whatever
is defined in requests).

**memory requests**

used for finding nodes with enough memory and making better scheduling decisions.

**cpu requests**

maps to the docker flag --cpu-shares, which defines a relative weight of that container
for cpu time. the relative share is executed per core, which can lead to unexpected outcomes
but probably nothing to worry about in our use cases. a container will never be killed
because of this metric.

resource limits
---------------

limits define the upper bound of resources a container can use. limits must always be greater
equal than requests. the behaviour differs between cpu and memory.

**memory limits**

maps to the docker flag --memory, which means processes in the container get killed by the
kernel if they hit that memory usage. given you run one process per container this will kill
the whole container and kubernetes will try to restart it.

**cpu limits**

maps to the docker flag --cpu-quota, which limits CPU time of that container's processes.
seems like you can define that a container can only max utilize a core by e.g. 50%.

but, let's assume you have 3 of them on a single-core node this can lead to over-utilizing it.

conclusion
==========

* ``requests`` are for making scheduling decisions
* ``limits`` are real resource limits of containers
* effect of cpu limits still fuzzy to me
* choosing higher ``limits`` than ``requests`` allows to over-provision nodes,
  but has the danger of over-utilizing it
* ``requests`` are required for using the horizontal pod autoscaler
