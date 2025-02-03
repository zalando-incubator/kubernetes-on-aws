package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/ingress"
	admissionapi "k8s.io/pod-security-admission/api"
	"net/http"
	"time"
)

var _ = Describe("Ingress tests for OPA filters", func() {
	f := framework.NewDefaultFramework("skipper-ingress-with-opa")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline

	var (
		cs            kubernetes.Interface
		jig           *ingress.TestJig
		ingressCreate *netv1.Ingress
		ns            string
		hostName      string
		port          int
		serviceName   = "aaas-infrastructure-tests"
		opaPolicyName = "aaas-infrastructure-tests"
	)

	BeforeEach(func() {
		jig = ingress.NewIngressTestJig(f.ClientSet)
		cs = f.ClientSet
		ns = f.Namespace.Name
		hostName = fmt.Sprintf("%s-%d.%s", serviceName, time.Now().UTC().Unix(), E2EHostedZone())
		port = 8080

		ingressCreate = createIngressWithInfo(serviceName, hostName, ns, port, cs, jig)
	})

	It("opaAuthorizeRequest filter should pass through authorized requests [Ingress] [Opa]", func() {
		// Authorization done with the rule
		// https://github.bus.zalan.do/corporate-iam/aaas-infrastructure-tests-policies/blob/main/bundle/policy/ingress/rules.rego

		authorizationEnforcedPath := "/auth"
		path := "/"
		ingressRoute := fmt.Sprintf(`authorize: PathRegexp("%s") -> modPath("%s", "%s") -> opaAuthorizeRequest("%s") -> inlineContent("Got response") -> <shunt>;`, authorizationEnforcedPath, authorizationEnforcedPath, path, opaPolicyName)
		ingressUpdate := updateIngressAndWait(serviceName, hostName, path, ingressRoute, port, ingressCreate, cs)

		By(fmt.Sprintf("Calling ingress %s/%s we wait to get a 403 with opaAuthorizeRequest %s policy", ingressUpdate.Namespace, ingressUpdate.Name, opaPolicyName))
		rt, quit := createHTTPRoundTripper()
		defer func() {
			quit <- struct{}{}
		}()

		url := "https://" + hostName + authorizationEnforcedPath
		req, err := http.NewRequest("GET", url, nil)
		resp, err := getAndWaitResponseWithInterval(rt, req, 60*time.Second, 5*time.Second, http.StatusForbidden)
		framework.ExpectNoError(err)
		Expect(resp.StatusCode).To(Equal(http.StatusForbidden))

		By(fmt.Sprintf("Calling ingress %s/%s we wait to get a 200 with opaAuthorizeRequest %s policy", ingressUpdate.Namespace, ingressUpdate.Name, opaPolicyName))
		req.Header.Set("Authorization", "Basic valid_token") // Authorized request
		resp, err = getAndWaitResponseWithInterval(rt, req, 60*time.Second, 5*time.Second, http.StatusOK)
		framework.ExpectNoError(err)
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		s, err := getBody(resp)
		framework.ExpectNoError(err)
		Expect(s).To(Equal("Got response"))
	})

	It("opaServeResponse filter should return body [Ingress] [Opa]", func() {
		// Authorization done with the rule
		// https://github.bus.zalan.do/corporate-iam/aaas-infrastructure-tests-policies/blob/main/bundle/policy/ingress/permissions.rego

		serveResponsePath := "/permissions"
		expectedPermissionsContent := "\"permissions\":{\"permission1\":{},\"permission2\":{}}"
		path := serveResponsePath
		ingressRoute := fmt.Sprintf(`authorize: PathRegexp("%s") -> opaServeResponse("%s") -> <shunt>;`, serveResponsePath, opaPolicyName)
		ingressUpdate := updateIngressAndWait(serviceName, hostName, path, ingressRoute, port, ingressCreate, cs)

		By(fmt.Sprintf("Calling ingress %s/%s we wait to get a 403 with opaServeResponse %s policy", ingressUpdate.Namespace, ingressUpdate.Name, opaPolicyName))
		rt, quit := createHTTPRoundTripper()
		defer func() {
			quit <- struct{}{}
		}()

		url := "https://" + hostName + serveResponsePath
		req, err := http.NewRequest("GET", url, nil)
		resp, err := getAndWaitResponseWithInterval(rt, req, 60*time.Second, 5*time.Second, http.StatusForbidden)
		framework.ExpectNoError(err)
		Expect(resp.StatusCode).To(Equal(http.StatusForbidden))

		By(fmt.Sprintf("Calling ingress %s/%s we wait to get a 200 with opaServeResponse %s policy", ingressUpdate.Namespace, ingressUpdate.Name, opaPolicyName))
		req.Header.Set("Authorization", "Basic permissions_token") // Authorized request
		resp, err = getAndWaitResponseWithInterval(rt, req, 60*time.Second, 5*time.Second, http.StatusOK)
		framework.ExpectNoError(err)
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		s, err := getBody(resp)
		framework.ExpectNoError(err)
		Expect(s).To(ContainSubstring(expectedPermissionsContent))
	})

})

func createIngressWithInfo(serviceName, hostName, ns string, port int, cs kubernetes.Interface, jig *ingress.TestJig) *netv1.Ingress {
	labels := map[string]string{"app": serviceName}
	waitTime := 10 * time.Minute

	// Create initial ingress
	ing := createIngress(serviceName, hostName, ns, "/", netv1.PathTypeImplementationSpecific, labels, nil, port)
	var err error
	ingressCreate, err := cs.NetworkingV1().Ingresses(ns).Create(context.TODO(), ing, metav1.CreateOptions{})
	framework.ExpectNoError(err)

	// Wait for ingress address
	addr, err := jig.WaitForIngressAddress(context.TODO(), cs, ns, ingressCreate.Name, waitTime)
	framework.ExpectNoError(err)

	// Ensure ingress exists
	_, err = cs.NetworkingV1().Ingresses(ns).Get(context.TODO(), ing.Name, metav1.GetOptions{ResourceVersion: "0"})
	framework.ExpectNoError(err)

	By("Waiting for LB to create endpoint and skipper route")
	err = waitForResponse(addr, "https", waitTime, isNotFound, true)
	framework.ExpectNoError(err)

	return ingressCreate
}

func updateIngressAndWait(serviceName, hostName, path, ingressRoute string, port int, ingressCreate *netv1.Ingress, cs kubernetes.Interface) *netv1.Ingress {
	updatedIng := updateIngress(ingressCreate.ObjectMeta.Name,
		ingressCreate.ObjectMeta.Namespace,
		hostName,
		serviceName,
		path,
		netv1.PathTypeImplementationSpecific,
		ingressCreate.ObjectMeta.Labels,
		map[string]string{
			"zalando.org/skipper-routes": ingressRoute,
		},
		port,
	)
	ingressUpdate, err := cs.NetworkingV1().Ingresses(ingressCreate.ObjectMeta.Namespace).Update(context.TODO(), updatedIng, metav1.UpdateOptions{})
	framework.ExpectNoError(err)
	time.Sleep(2 * time.Minute) // wait for routing change propagation

	return ingressUpdate
}
