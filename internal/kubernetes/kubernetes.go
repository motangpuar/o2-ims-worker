package kubeclient

import (
	"strings"
	"os"
	"context"
	"path/filepath"
	"fmt"
	"log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	cs kubernetes.Interface
}

func New(kubeconfigPath string) (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("[FAIL] build config: %w", err)
	}
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("[FAIL] create clientset: %w", err)
	}
	return &Client{cs: cs}, nil
}

// Cluster

type ClusterInfo struct {
	ClusterID     string
	NodeCount     int
	Namespaces    []string
	ServerVersion string
	Events        []EventInfo
	Pods          []PodInfo
	Nodes         []NodeInfo
}

type Clusters struct {
	Cluster map[string]*Cluster
}

type Cluster struct{
	ID string
	KubeConfig string
	Token string
	APIEndpoint string
	Info *ClusterInfo
	Ready bool
	Nodes []struct{
		Node string
		Role string
		OS string
		IP string
	}
}

var PtrClusterMap *Clusters

func InitCluster(id string, nodes *[]struct{
	Node string
	Role string
	OS string
	IP string
}){
	cwd, err := os.Getwd()
	if err != nil {
		return 
	}

	if PtrClusterMap == nil {
		PtrClusterMap = &Clusters{
			Cluster: make(map[string]*Cluster),
		}
	}

	for _,n := range(*nodes){
		if n.Role == "master" {
			macAsID := strings.ReplaceAll(n.Node, ":", "-")
    		filePattern := "kubeconfig-"+macAsID+".yaml"
			kubeconfigDest := filepath.Join(cwd, "assets", "ansible", filePattern)
			PtrClusterMap.Cluster[id] = &Cluster{
				ID: id,
				Nodes: *nodes,
				KubeConfig: kubeconfigDest,
			}
		}
	}
	log.Printf("[KUBERNETES] Created cluster %v", PtrClusterMap)
}


//
// This function should use kubeconfig path from its own self 
// Not hard pass from arguments
//
func GetClusters(ctx context.Context) (*Clusters,error) {
	for key,cluster := range PtrClusterMap.Cluster {
		log.Printf("[KUBERNETES] Fetch using kubeconfig %s", cluster.KubeConfig)
		kc, err := New(cluster.KubeConfig)
		if err != nil {
			log.Printf("[KUBERNETES] Error using kubeconfig")
			PtrClusterMap.Cluster[key].Ready = false
			continue
			//return PtrClusterMap, err
		} else {
			PtrClusterMap.Cluster[key].Ready = true
		}
		info,err := kc.ClusterInfo(ctx)
		if err != nil {
			log.Printf("[KUBERNETS] Failed to fetch clusterinfo")
			return nil,err
		}
		log.Printf("[KUBERNETES] Current Cluster Info: %+v", info)
		// Populate cluster info
		PtrClusterMap.Cluster[key].Info = info
	}
	return PtrClusterMap, nil
}

// Nodes
type NodeInfo struct {
	Name             string
	Status           string
	Roles            []string
	OS               string
	KernelVersion    string
	ContainerRuntime string
	Architecture     string
	CPUCapacity      string
	MemoryCapacity   string
	CPUAllocatable   string
	MemoryAllocatable string
	InternalIP       string
	Labels           map[string]string
}

func (c *Client) ClusterInfo(ctx context.Context) (*ClusterInfo, error) {
	nodes, err := c.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("[FAIL] list nodes: %w", err)
	}

	nsList, err := c.cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("[FAIL] list namespaces: %w", err)
	}

	version, err := c.cs.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("[FAIL] server version: %w", err)
	}

	namespaces := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		namespaces = append(namespaces, ns.Name)
	}

	nodeObjects := make([]NodeInfo, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		nodeObjects = append(nodeObjects, parseNode(node))
	}

	// Populate Pods
	var podObjects []PodInfo
	for _, ns := range namespaces {
		podItems, err := c.Pods(ctx, ns)
		if err != nil {
			log.Printf("[KUBERNETES] Faile to get pod data")
			return nil, err
		}
		podObjects = append(podObjects, podItems...)
	}

	// Populate Events
	var eventObjects []EventInfo
	for _, ns := range namespaces {
		eventItems, err := c.Events(ctx, ns)
		if err != nil {
			log.Printf("[KUBERNETES] Faile to get event data")
			return nil, err
		}
		eventObjects = append(eventObjects, eventItems...)
	}

	return &ClusterInfo{
		NodeCount:     len(nodes.Items),
		Namespaces:    namespaces,
		ServerVersion: version.GitVersion,
		Events:        eventObjects,
		Pods:          podObjects,
		Nodes:         nodeObjects,
	}, nil
}


func (c *Client) Nodes(ctx context.Context) ([]NodeInfo, error) {
	nodes, err := c.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("[FAIL] list nodes: %w", err)
	}

	result := make([]NodeInfo, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		result = append(result, parseNode(node))
	}
	return result, nil
}

func (c *Client) Node(ctx context.Context, name string) (*NodeInfo, error) {
	node, err := c.cs.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("[FAIL] get node %s: %w", name, err)
	}
	info := parseNode(*node)
	return &info, nil
}

func parseNode(node corev1.Node) NodeInfo {
	status := "NotReady"
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			status = "Ready"
		}
	}

	roles := []string{}
	for label := range node.Labels {
		if label == "node-role.kubernetes.io/control-plane" {
			roles = append(roles, "control-plane")
		}
		if label == "node-role.kubernetes.io/worker" {
			roles = append(roles, "worker")
		}
	}
	if len(roles) == 0 {
		roles = append(roles, "worker")
	}

	internalIP := ""
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			internalIP = addr.Address
		}
	}

	return NodeInfo{
		Name:              node.Name,
		Status:            status,
		Roles:             roles,
		OS:                node.Status.NodeInfo.OperatingSystem,
		KernelVersion:     node.Status.NodeInfo.KernelVersion,
		ContainerRuntime:  node.Status.NodeInfo.ContainerRuntimeVersion,
		Architecture:      node.Status.NodeInfo.Architecture,
		CPUCapacity:       node.Status.Capacity.Cpu().String(),
		MemoryCapacity:    node.Status.Capacity.Memory().String(),
		CPUAllocatable:    node.Status.Allocatable.Cpu().String(),
		MemoryAllocatable: node.Status.Allocatable.Memory().String(),
		InternalIP:        internalIP,
		Labels:            node.Labels,
	}
}

type PodInfo struct {
	Name      string
	Namespace string
	Status    string
	Node      string
	IP        string
	Images    []string
}

func (c *Client) Pods(ctx context.Context, namespace string) ([]PodInfo, error) {
	pods, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("[FAIL] list pods: %w", err)
	}

	result := make([]PodInfo, 0, len(pods.Items))
	for _, pod := range pods.Items {
		images := make([]string, 0, len(pod.Spec.Containers))
		for _, c := range pod.Spec.Containers {
			images = append(images, c.Image)
		}
		result = append(result, PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    string(pod.Status.Phase),
			Node:      pod.Spec.NodeName,
			IP:        pod.Status.PodIP,
			Images:    images,
		})
	}
	return result, nil
}

// Services

type ServiceInfo struct {
	Name      string
	Namespace string
	Type      string
	ClusterIP string
	Ports     []string
}

func (c *Client) Services(ctx context.Context, namespace string) ([]ServiceInfo, error) {
	svcs, err := c.cs.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("[FAIL] list services: %w", err)
	}

	result := make([]ServiceInfo, 0, len(svcs.Items))
	for _, svc := range svcs.Items {
		ports := make([]string, 0, len(svc.Spec.Ports))
		for _, p := range svc.Spec.Ports {
			ports = append(ports, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
		}
		result = append(result, ServiceInfo{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Type:      string(svc.Spec.Type),
			ClusterIP: svc.Spec.ClusterIP,
			Ports:     ports,
		})
	}
	return result, nil
}

// Events

type EventInfo struct {
	Namespace string
	Name      string
	Reason    string
	Message   string
	Count     int32
	Type      string
}

func (c *Client) Events(ctx context.Context, namespace string) ([]EventInfo, error) {
	events, err := c.cs.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("[FAIL] list events: %w", err)
	}

	result := make([]EventInfo, 0, len(events.Items))
	for _, e := range events.Items {
		result = append(result, EventInfo{
			Namespace: e.Namespace,
			Name:      e.Name,
			Reason:    e.Reason,
			Message:   e.Message,
			Count:     e.Count,
			Type:      e.Type,
		})
	}
	return result, nil
}
