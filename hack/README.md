# Vagrant K8s cluster

The scripts in this folder is used for setting up a cluster that mimics the AWS
setup to some extend to make it easier to test iterate on different setups.

The base setup consists of 3 nodes, one etcd node, one controller node and one
worker node. The three cloud-configs are used to configure the nodes
respectively.

```
+----------------+
|etcd (e1)       |
|172.17.4.51     |
|                |
+----------------+
        ^
        |
+----------------+       +---------------+
|controller (c1) |       |worker (w1)    |
|172.17.4.101    |<------|172.17.4.201   |
|                |       |               |
+----------------+       +---------------+
```

### Setup cluster

1. Generate certificates used for worker -> master and user -> master
    communication.
    ```sh
    $ ./gen_certs.sh
    ```

2. Generate userdata from the cloud-config templates
   ```sh
   $ go run render_userdata.go
   ```

3. Start the VMs with vagrant
   ```sh
   $ vagrant up
   ```
   Once the machines are running you can ssh into one of the nodes with:
   ```sh
   $ vagrant ssh c1 # c1 is for the controller node, use w1 for worker.
   ```

4. After some time downloading container images, everything should be setup:
    ```sh
    $ kubectl --kubeconfig kubeconfig get pod --all-namespaces
    NAMESPACE     NAME                                   READY     STATUS    RESTARTS   AGE
kube-system   heapster-v1.2.0-4088228293-0bfka       2/2       Running   0          59m
kube-system   kube-apiserver-172.17.4.101            1/1       Running   0          1h
kube-system   kube-controller-manager-172.17.4.101   1/1       Running   0          1h
kube-system   kube-dns-v19-36nbh                     3/3       Running   0          1h
kube-system   kube-proxy-172.17.4.101                1/1       Running   0          1h
kube-system   kube-proxy-172.17.4.201                1/1       Running   0          1h
kube-system   kube-scheduler-172.17.4.101            1/1       Running   2          1h
kube-system   kubernetes-dashboard-v1.4.0-ugnlc      1/1       Running   0          1h
    ```

