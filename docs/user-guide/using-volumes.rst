==================
Persistent Storage
==================

Some of your pods need to persist data across pod restarts (e.g. databases). In order to facilitate this we can mount
folders into our pods that are backed by EBS volumes on AWS.

Deploying Redis
===============

In this example we're going to deploy a non high-available but persistent Redis container.

We start out by deploying a non-persistent version first and then extend it to keep our data across pod and node
restarts. Submit the following two manifests to your cluster to create a deployment and a service for your redis
instance.

.. code-block:: yaml

    apiVersion: v1
    kind: Service
    metadata:
      name: redis
    spec:
      ports:
      - port: 6379
        targetPort: 6379
      selector:
        application: redis

.. code-block:: yaml

    apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: redis
    spec:
      replicas: 1
      template:
        metadata:
          labels:
            application: redis
        spec:
          containers:
          - name: redis
            image: redis:3.2.5

Your service can be accessed from other pods by using the automatically generated cluster-internal DNS name or service
IP address. So given you use the manifests as printed above and you're running in the default
namespace you should find your Redis instance at ``redis.default.svc.cluster.local`` from any other pod.

You can run an interactive pod and test that it works. You can use the same Redis image as it contains the redis CLI.

.. code-block:: none

    $ zkubectl run redis-cli --rm -ti --image=redis:3.2.5 --restart=Never /bin/bash
    $ redis-cli -h redis.default.svc.cluster.local
    redis-default.hackweek.zalan.do:6379> quit

Creating a volume
-----------------

There's one major problem with your Redis container: It lacks some persistent storage. So let's add it.

We'll be using something that's called a ``PersistentVolumeClaim``. Claims are an abstraction over the actual
storage system in your cluster. With a claim you define that you need some amount of storage at some path inside your
container. Based on your needs the cluster management system will provision you some storage out of its available
storage pool. In case of AWS you usually get an EBS volume attached to the node and mounted into your container.

Submit the following file to your cluster in order to claim 10GB of standard storage.

.. code-block:: yaml

    apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      name: redis-data
      annotations:
        volume.beta.kubernetes.io/storage-class: standard
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 10Gi

``standard`` is a storage class that we defined in the cluster. It's implemented via an SSD-EBS volume.
``ReadWriteOnce`` means that this storage can only be attached to one instance at a time. Both of these values can be
safely ignored, more important for you are the name and the requested size of storage.

After submitting the manifest to the cluster you can list your storage claims:

.. code-block:: none

    $ zkubectl get persistentVolumeClaims
    NAME            STATUS    VOLUME                                     CAPACITY   ACCESSMODES   AGE
    redis-data      Bound     pvc-fc26de82-b577-11e6-b2a5-02c15a33e7b7   10Gi       RWO           4s

Status ``Bound`` means that your claim was successfully implemented and is now bound to a persistent volume. You can
also list all volumes:

.. code-block:: none

    $ zkubectl get persistentVolumes
    NAME                                       CAPACITY   ACCESSMODES   RECLAIMPOLICY   STATUS    CLAIM                      REASON    AGE
    pvc-fc26de82-b577-11e6-b2a5-02c15a33e7b7   10Gi       RWO           Delete          Bound     default/redis-data                   8m

If you want to dig deeper you can describe the volume and see that it's backed by an EBS volume.

.. code-block:: none

    $ zkubectl describe persistentVolume pvc-fc26de82-b577-11e6-b2a5-02c15a33e7b7
    Name:		pvc-fc26de82-b577-11e6-b2a5-02c15a33e7b7
    Labels:		failure-domain.beta.kubernetes.io/region=eu-central-1
        failure-domain.beta.kubernetes.io/zone=eu-central-1b
    Status:		Bound
    Claim:		default/redis-data
    Reclaim Policy:	Delete
    Access Modes:	RWO
    Capacity:	10Gi
    Message:
    Source:
        Type:	AWSElasticBlockStore (a Persistent Disk resource in AWS)
        VolumeID:	aws://eu-central-1b/vol-a36c7039
        FSType:	ext4
        Partition:	0
        ReadOnly:	false
    No events.

Here, you can also see in which zone the EBS volume was created. Any pod that wants to mount this volume must be
scheduled to a node running in that same zone. Luckily, Kubernetes takes care of that.

Attaching a volume to a pod
---------------------------

Modify your deployment in the following way in order to use the persistent volume claim we created above.

.. code-block:: yaml

    apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: redis
    spec:
      replicas: 1
      template:
        metadata:
          labels:
            application: redis
        spec:
          containers:
          - name: redis
            image: redis:3.2.5
            volumeMounts:
            - mountPath: /data
              name: redis-data
          volumes:
            - name: redis-data
              persistentVolumeClaim:
                claimName: redis-data

We did two things here: First we registered the ``persistentVolumeClaim`` under the ``volumes`` section in the pod
definition and gave it a name. Then, by using the name, we mounted that volume under a path in the container in the
``volumeMounts`` section. The reason for having a two-level definition here is because multiple containers in the same
pod can mount the same volume under different paths, e.g. for sharing data.

Secondly, our Redis container uses ``/data`` to store its data which is where we mounted our persistent volume.
This way, anything that Redis stores will be written to the EBS volume and thus can be mounted on another node in case
of node failure.

Note, that you usually want ``replicas`` to be ``1`` when using this approach. Though, you can use more replicas which
would result in many pods mounting the same volume. As this volume is backed by an EBS volume this forces Kubernetes
to schedule all replicas on the same node. If you require multiple replicas, each with their own persistent volume,
you should rather think about using a ``StatefulSet`` instead.

Trying it out
-------------

Find out where your pod currently runs:

.. code-block:: none

    $ zkubectl get pods -o wide
      NAME                        READY     STATUS    RESTARTS   AGE       IP          NODE
      redis-3548935762-qevsk      1/1       Running   0          2m        10.2.1.66   ip-172-31-15-65.eu-central-1.compute.internal

The node it landed on is ``ip-172-31-15-65.eu-central-1.compute.internal``. Connect to your Redis endpoint and create some data:

.. code-block:: none

    $ zkubectl run redis-cli --rm -ti --image=redis:3.2.5 --restart=Never /bin/bash
    $ redis-cli -h redis.default.svc.cluster.local
    redis-default.hackweek.zalan.do:6379> set foo bar
    OK
    redis-default.hackweek.zalan.do:6379> get foo
    "bar"
    redis-default.hackweek.zalan.do:6379> quit

Simulate a pod failure by deleting your pod. This will make Kubernetes create a new one potentially on another
node but always in the same zone due to using an EBS volume.

.. code-block:: none

    $ zkubectl delete pod redis-3548935762-qevsk
    pod "redis-3548935762-qevsk" deleted

    $ zkubectl get pods -o wide
    NAME                        READY     STATUS    RESTARTS   AGE       IP          NODE
    redis-3548935762-p4z9y      1/1       Running   0          1m        10.2.72.2   ip-172-31-10-115.eu-central-1.compute.internal

In this example the new pod landed on another node (``ip-172-31-10-115.eu-central-1.compute.internal``).
Let's check that it's available and didn't loose any data. Connect to Redis in the same way as before.

.. code-block:: none

    $ zkubectl run redis-cli --rm -ti --image=redis:3.2.5 --restart=Never /bin/bash
    $ redis-cli -h redis.default.svc.cluster.local
    redis-default.hackweek.zalan.do:6379> get foo
    "bar"
    redis-default.hackweek.zalan.do:6379> quit

And indeed, everything is still there.

Deleting a volume
-----------------

All it takes to delete a volume is to delete the corresponding claim that initiated its creation in the first place.

.. code-block:: none

    $ zkubectl delete persistentVolumeClaim redis-data
    persistentvolumeclaim "redis-data" deleted

To fully clean up after yourself also delete the deployment and the service:

.. code-block:: none

    $ zkubectl delete deployment,service redis
    service "redis" deleted
    deployment "redis" deleted

Additional resources
====================

* http://kubernetes.io/docs/user-guide/volumes/
* http://kubernetes.io/docs/user-guide/persistent-volumes/
