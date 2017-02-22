=====================
Running in Production
=====================

Minimum Number of Replicas
==========================

Always run at least two replicas of your application to survive cluster updates and autoscaling without downtime.


Readiness Probes
================

Configure a ``readinessProbe`` to make sure that your container only gets traffic after successful application start:

.. code-block:: yaml

      containers:
      - name: mycontainer
        image: myimage
        readinessProbe:
          httpGet:
            # Path to probe; should be cheap, but representative of typical behavior
            path: /.well-known/health
            port: 8080
          initialDelaySeconds: 10
          timeoutSeconds: 1


Resource Requests
=================

Always configure resource requests for both CPU and memory.
The Kubernetes scheduler and cluster autoscaler need this information in order to make the right decisions.


Resource Limits
===============

You should configure resource limits if possible. The memory resource limit will get your container ``OOMKilled`` when reaching the limit.
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
