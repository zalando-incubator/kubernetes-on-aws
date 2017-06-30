=====================
Running in Production
=====================

Number of Replicas
==================

Always run at least two replicas (three or more are recommended) of your application to survive cluster updates and autoscaling without downtime.

Readiness Probes
================

Web applications should always configure a ``readinessProbe`` to make sure that the container only gets traffic after a successful startup:

.. code-block:: yaml

      containers:
      - name: mycontainer
        image: myimage
        readinessProbe:
          httpGet:
            # Path to probe; should be cheap, but representative of typical behavior
            path: /.well-known/health
            port: 8080
          timeoutSeconds: 1

See `Configuring Liveness and Readiness Probes`_ for details.

.. _Configuring Liveness and Readiness Probes: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-probes/

Resource Requests
=================

Always configure `resource requests`_ for both CPU and memory.
The Kubernetes scheduler and cluster autoscaler need this information in order to make the right decisions.
Example:


.. code-block:: yaml

    containers:
      - name: mycontainer
        image: myimage
        resources:
          requests:
            cpu: 100m     # 100 millicores
            memory: 200Mi # 200 MiB

.. _resource requests: https://kubernetes.io/docs/user-guide/compute-resources/

Resource Limits
===============

You should configure a resource limit for memory if possible. The memory resource limit will get your container ``OOMKilled`` when reaching the limit.
Set the JVM heap memory dynamically by using the ``java-dynamic-memory-opts`` script from Zalando's OpenJDK base image and setting ``MEM_TOTAL_KB`` to ``limits.memory``:

.. code-block:: yaml

    containers:
      - name: mycontainer
        image: myjvmdockerimage
        env:
          # set the maximum available memory as JVM would assume host/node capacity otherwise
          # this is evaluated by java-dynamic-memory-opts in the Zalando OpenJDK base image
          # see https://github.com/zalando/docker-openjdk
          - name: MEM_TOTAL_KB
            valueFrom:
              resourceFieldRef:
                resource: limits.memory
                divisor: 1Ki
        resources:
          requests:
            cpu: 100m
            memory: 2Gi
          limits:
            memory: 2Gi
