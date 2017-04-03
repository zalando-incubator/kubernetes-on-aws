===================================
ADR-004: Roles and Service Accounts
===================================

Context
=======

We need to define roles and service accounts to allow all our use cases. Our first concerns are to allow the following:

- Users should be able to deploy (manually in test cluster, via the deploy API in production clusters), but we do not want them by default to read secrets
- Admins should get full access to all resources, mostly for emergency access
- Applications should not get by default write access to the Kubernetes API
- It should be possible for some applications to write to the Kubernetes API.

Decision
========

We define the following Roles:

* ReadOnly: allowed to read every resource, but not secrets. "exec"
  and "proxy" and similar operations are not allowed. Allowed to do
  "port-forward" to special proxy, which will enable DB access.
* PowerUser: "restricted" Pod Security Policy with write access to all
  namespaces but kube-system, ReadOnly access to kube-system
  namespace, "exec" and "proxy" are allowed, RW for secrets, no write
  of daemonsets. DB access through "port-forward" and special proxy.
* Operator: "privileged" Pod Security Policy with write access to the
  own namespace and read and write access to third party resources in
  all namespaces.
* Controller: Kubernetes component controller-manager is not allowed
  to "use" other Pod Security Policies then "restricted", such that
  serviceAccount authorization is used to check the permission. To all
  other resources it has full access.
* Admin: full access to all resources

And the following pairs <namespace, service account> that will get the listed role, assigned by the WebHook:

* "kube-system:default" - Admin
* "default:default" - ReadOnly
* "\*:operator" - Operator
* kube-controller-manager - Controller
* kubelet - Admin

Application that will want write access to the Kubernetes API will have to use the "operator" service account.

Status
======

Accepted.

Consequences
============

This decision is a breaking change of what was previously defined for
applications. Users that need applications with write access to the
Kubernetes API will need to select the right service account.
The controller-manager has now an identity and uses the secured
kube-apiserver endpoint, such that it can be authorized by the webhook.
