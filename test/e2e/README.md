# E2E tests for Kubernetes on AWS

This directory contains e2e tests for the kubernetes-on-aws configuration.

It is based on the test framework used by Kubernetes itself. See [Kubernetes
e2e tests](https://github.com/kubernetes/kubernetes/tree/master/test/e2e) for
examples of how to write the tests or checkout the files already defined e.g.
`external_dns.go`.

## Running the tests

1. First you need [ginkgo] which is used to orchestrate the tests:

  ```bash
  make deps
  ```

2. Build the e2e test binary:

  ```bash
  make
  ```

3. Run the e2e tests

  ```bash
  KUBECONFIG=~/.kube/config HOSTED_ZONE=example.org CLUSTER_ALIAS=example \
    ginkgo -nodes=1 -flakeAttempts=2 \
    -focus="(\[Conformance\]|\[StatefulSetBasic\]|\[Feature:StatefulSet\]\s\[Slow\].*mysql|\[Zalando\])" \
    -skip="(\[Serial\])" \
    "e2e.test" -- -delete-namespace-on-failure=false -non-blocking-taints=node.kubernetes.io/role
  ```

  Where `~/.kube/config` is pointing to the cluster you want to run the tests
  against, `HOSTED_ZONE` is the hosted zone configured for the cluster, and
  `CLUSTER_ALIAS` is the cluster's user-friendly name.

  This will run all the tests we normally run on a PR, you can single out tests
  by tweaking the values of the focus/skip flags.

## How to write a test

Tests are using [Ginkgo](https://github.com/onsi/ginkgo) as BDD test framework and
[Gomega](https://godoc.org/github.com/onsi/gomega) as matcher library.
Helper functions to create Kubernetes types are found in `util.go`.
Look at the current tests as examples `external_dns.go` and `psp.go` and make sure you have the right imports.

### Create a new test for Kubernetes type Foo

Simple test template that shows how you can create a new file from
scratch and test the Kubernetes type Foo.

```go
  package e2e

  import (
  	. "github.com/onsi/ginkgo"
  	. "github.com/onsi/gomega"

  	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

  	"k8s.io/client-go/kubernetes"
  	"k8s.io/kubernetes/test/e2e/framework"
  )

  var _ = describe("Thing under test, func() {
  	f := framework.NewDefaultFramework("Describe thing under test")
  	var cs kubernetes.Interface
      // we need always a clean clientset.Interface for all our tests
  	BeforeEach(func() {
  		cs = f.ClientSet
  	})

  	It("Should create a test Foo [Foo] [Zalando]", func() {
              name := "foo"
  		ns := f.Namespace.Name
  		labels := map[string]string{
  			"app": name,
  		}

              // write a message to the user
  		By("Creating foo " + name + " in namespace " + ns)
              // creates Kubernetes foo type, function createFoo() can be found in util.go
  		foo := createFoo(name, ns, labels)
              // cleanup
  		defer func() {
  			By("deleting the foo)
  			defer GinkgoRecover()
  			err2 := cs.CoreV1().Foo(ns).Delete(foo.Name, metav1.NewDeleteOptions(0))
  			Expect(err2).NotTo(HaveOccurred())
  		}()
              // creates the Ingress Object
  		_, err := cs.CoreV1().Foo(ns).Create(foo)
  		Expect(err).NotTo(HaveOccurred())
  	})
  })
```


### Create a POD

```go
  targetPort := 80
  name := "foo"
  nameprefix := name + "-"
  ns := f.Namespace.Name
  labels := map[string]string{
  	"app": name,
  }

  // POD
  By("Creating a POD with prefix " + nameprefix + " in namespace " + ns)
  pod := createNginxPod(nameprefix, ns, labels, targetPort)
  defer func() {
  	By("deleting the pod")
  	defer GinkgoRecover()
  	err2 := cs.CoreV1().Pods(ns).Delete(pod.Name, metav1.NewDeleteOptions(0))
  	Expect(err2).NotTo(HaveOccurred())
  }()
  _, err = cs.CoreV1().Pods(ns).Create(pod)
  Expect(err).NotTo(HaveOccurred())
  framework.ExpectNoError(f.WaitForPodRunning(pod.Name))
```

### Create a SVC

```go
  port := 83
  targetPort := 80
  serviceName := "foo"
  ns := f.Namespace.Name
  labels := map[string]string{
  	"app": serviceName,
  }
  // SVC
  By("Creating service " + serviceName + " in namespace " + ns)
  service := createServiceTypeClusterIP(serviceName, labels, port, targetPort)
  defer func() {
  	By("deleting the service")
  	defer GinkgoRecover()
  	err2 := cs.CoreV1().Services(ns).Delete(service.Name, metav1.NewDeleteOptions(0))
  	Expect(err2).NotTo(HaveOccurred())
  }()
  _, err := cs.CoreV1().Services(ns).Create(service)
  Expect(err).NotTo(HaveOccurred())
```

### Create a Ingress and wait for external components to be created

Create Kubernetes ingress object:

```go
  port := 83
  serviceName := "foo"
  hostName := serviceName + ".teapot-e2e.zalan.do"
  ns := f.Namespace.Name
  labels := map[string]string{
  	"app": serviceName,
  }

  // Ingress
  By("Creating an ingress with name " + serviceName + " in namespace " + ns + " with hostname " + hostName)
  ing := createIngress(serviceName, hostName, ns, labels, port)
  defer func() {
  	By("deleting the ingress")
  	defer GinkgoRecover()
  	err2 := cs.NetworkingV1beta1().Ingresses(ns).Delete(ing.Name, metav1.NewDeleteOptions(0))
  	Expect(err2).NotTo(HaveOccurred())
  }()
  ingressCreate, err := cs.NetworkingV1beta1().Ingresses(ns).Create(ing)
  Expect(err).NotTo(HaveOccurred())
  addr, err := jig.WaitForIngressAddress(cs, ns, ingressCreate.Name, 3*time.Minute)
  Expect(err).NotTo(HaveOccurred())
  ingress, err := cs.NetworkingV1beta1().Ingresses(ns).Get(ing.Name, metav1.GetOptions{ResourceVersion: "0"})
  Expect(err).NotTo(HaveOccurred())
  By(fmt.Sprintf("ALB endpoint from ingress status: %s", ingress.Status.LoadBalancer.Ingress[0].Hostname))
```

Follow up code, that waits for creations to be happen:

```go
  // skipper http -> https redirect
  By("Waiting for skipper route to default redirect from http to https, to see that our ingress-controller and skipper works")
  err = waitForResponse(addr, "http", 2*time.Minute, 301, true)
  Expect(err).NotTo(HaveOccurred())
  // ALB ready
  By("Waiting for ALB to create endpoint " + addr + " and skipper route, to see that our ingress-controller and skipper works")
  err = waitForResponse(addr, "https", 2*time.Minute, 200, true) // insecure=true
  Expect(err).NotTo(HaveOccurred())
  // DNS ready
  By("Waiting for DNS to see that mate and skipper route to service and pod works")
  err = waitForResponse(hostName, "https", 2*time.Minute, 200, false)
  Expect(err).NotTo(HaveOccurred())
```

### FAQ

* **What is the fastest way to iterate on my test**
  Since the tests pull in all of kubernetes it can take some time to build the
  test binary.

  To build it simply run:

  ```bash
  make
  ```

  This will setup the go modules correctly and build a binary
  `e2e.test`.

  Run all Zalando tests from your local build:

  ```bash
  # S3_AWS_IAM_BUCKET and AWS_IAM_ROLE is required for the AWS-IAM tests.
  KUBECONFIG=~/.kube/config HOSTED_ZONE=example.org CLUSTER_ALIAS=example \
  S3_AWS_IAM_BUCKET=zalando-e2e-aws-iam-test-12345678912-kube-1 \
  AWS_IAM_ROLE=kube-1-e2e-aws-iam-test \
  ginkgo -nodes=25 -flakeAttempts=2 -focus="\[Zalando\]" \
  e2e.test -- -non-blocking-taints=node.kubernetes.io/role,nvidia.com/gpu
  ```

* **Why is the go modules such a mess?**
  Because `Kubernetes` uses symlinks in its own vendor folder (e.g. `ln -s
  staging/src/k8s.io/client-go k8s.io/client-go`) we need to do something
  similar to make go modules understand what is going on.  We do this by
  `replace` directives in the `go.mod` file and the script `setup_modules.sh`
  to setup the replaces.

  > Just run `make` when you clone this repo and it should all work.
  >
  > - Mikkel Larsen

[ginkgo]: https://onsi.github.io/ginkgo/
