package main

import (
	"context"
	"errors"
	"log"
	"os"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	kubeClient, err := newClient()
	if err != nil {
		log.Fatalf("Failed to setup Kubernetes client: %v", err)
	}

	for {
		ctx := context.Background()
		candidates, err := candidateNodes(ctx, kubeClient)
		if err != nil {
			log.Printf("Failed to get candidate nodes: %v", err)
			continue
		}

		if len(candidates) == 0 {
			log.Printf("No nodes with old daemonset pods found, exiting")
			break
		}

		decomissioningNodes := 0
		for _, node := range candidates {
			if node.Node.Labels["lifecycle-status"] != "ready" {
				decomissioningNodes++
				continue
			}

			if err := decommissionNode(ctx, kubeClient, node); err != nil {
				log.Printf("Failed to decommission node %s: %v", node.Node.Name, err)
			}
			log.Printf("Marked node %s for decommissioning", node.Node.Name)
			decomissioningNodes++
		}

		log.Printf("Waiting for %d nodes with old daemonset pods to decommission", decomissioningNodes)
		time.Sleep(30 * time.Second)
	}

}

// newClient will try to create an in-cluster client if possible, otherwise create one with configuration from $KUBECONFIG or $HOME/.kube/config
func newClient() (kubernetes.Interface, error) {
	// first try to get client from kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.ExpandEnv("${HOME}/.kube/configs")
	}

	// if no kubeconfig is found, try in-cluster config
	_, err := os.Stat(kubeconfig)
	if err != nil {
		if e, ok := err.(*os.PathError); ok && errors.Is(e.Err, os.ErrNotExist) {
			// Try in-cluster config
			cfg, err := rest.InClusterConfig()
			if err != nil {
				return nil, err
			}
			return kubernetes.NewForConfig(cfg)
		}
		return nil, err
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

type Node struct {
	Pods []v1.Pod
	Node *v1.Node
}

func candidateNodes(ctx context.Context, client kubernetes.Interface) ([]*Node, error) {
	nodeMapping, err := nodeMapping(ctx, client)
	if err != nil {
		return nil, err
	}

	daemonsets, err := onDeleteDaemonsets(ctx, client)
	if err != nil {
		return nil, err
	}

	candidates := make([]*Node, 0, len(nodeMapping))
	for _, node := range nodeMapping {
		for _, p := range node.Pods {
			if oldDaemonsetPod(p, daemonsets) {
				candidates = append(candidates, node)
			}
		}
	}

	return candidates, nil
}

func oldDaemonsetPod(pod v1.Pod, onDeleteDaemonsets map[dsID]int64) bool {
	for _, owner := range pod.ObjectMeta.OwnerReferences {
		if owner.Kind != "DaemonSet" {
			continue
		}

		dsID := dsID{
			Name:      owner.Name,
			Namespace: pod.Namespace,
			UID:       owner.UID,
		}

		podGenStr, ok := pod.Labels["pod-template-generation"]
		if !ok {
			continue
		}

		podGen, err := strconv.ParseInt(podGenStr, 10, 64)
		if err != nil {
			continue
		}

		if gen, ok := onDeleteDaemonsets[dsID]; ok && podGen != gen {
			return true
		}
	}

	return false
}

func nodeMapping(ctx context.Context, client kubernetes.Interface) (map[string]*Node, error) {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	nodeMapping := map[string]*Node{}
	for _, node := range nodes.Items {
		n := node
		nodeMapping[node.Name] = &Node{
			Node: &n,
		}
	}

	pods, err := client.CoreV1().Pods(v1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		if node, ok := nodeMapping[pod.Spec.NodeName]; ok {
			// filter out failed/completed pods as they don't
			// consume any capacity on a node.
			switch pod.Status.Phase {
			case v1.PodSucceeded, v1.PodFailed:
				continue
			}
			node.Pods = append(node.Pods, pod)
		}
	}

	return nodeMapping, nil
}

type dsID struct {
	Name      string
	Namespace string
	UID       types.UID
}

func onDeleteDaemonsets(ctx context.Context, client kubernetes.Interface) (map[dsID]int64, error) {
	onDeleteDaemonsets := make(map[dsID]int64, 0)
	daemonsets, err := client.AppsV1().DaemonSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, ds := range daemonsets.Items {
		if ds.Spec.UpdateStrategy.Type == appsv1.OnDeleteDaemonSetStrategyType {
			onDeleteDaemonsets[dsID{
				Name:      ds.Name,
				Namespace: ds.Namespace,
				UID:       ds.UID,
			}] = ds.Generation
		}
	}

	return onDeleteDaemonsets, nil
}

func decommissionNode(ctx context.Context, client kubernetes.Interface, node *Node) error {
	taint := v1.Taint{
		Key:    "decommission-pending",
		Value:  "spot-replacement",
		Effect: v1.TaintEffectNoSchedule,
	}

	node.Node.Labels["lifecycle-status"] = "decommission-pending"
	node.Node.Spec.Taints = append(node.Node.Spec.Taints, taint)

	_, err := client.CoreV1().Nodes().Update(ctx, node.Node, metav1.UpdateOptions{})
	return err
}
