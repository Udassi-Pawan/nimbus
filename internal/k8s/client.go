package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Client talks to a Kubernetes cluster using the same kubeconfig as kubectl.
type Client struct {
	clientset   *kubernetes.Clientset
	contextName string
}

// NewClient loads kubeconfig and builds a Kubernetes API client.
// kubeconfigPath empty → default rules (~/.kube/config, KUBECONFIG env).
// context empty → current context from that file.
func NewClient(kubeconfigPath, context string) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}

	overrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		overrides.CurrentContext = context
	}

	rawConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).RawConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	contextName := rawConfig.CurrentContext
	if contextName == "" {
		return nil, fmt.Errorf("kubeconfig has no current context")
	}

	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("build rest config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}

	return &Client{
		clientset:   clientset,
		contextName: contextName,
	}, nil
}

func (c *Client) ContextName() string {
	return c.contextName
}

type ClusterStatus struct {
	Connected     bool   `json:"connected"`
	Context       string `json:"context"`
	ServerVersion string `json:"server_version,omitempty"`
	NodeCount     int    `json:"node_count"`
	NodesReady    int    `json:"nodes_ready"`
}

// Status checks that the API server responds and summarizes node health.
func (c *Client) Status(ctx context.Context) (ClusterStatus, error) {
	status := ClusterStatus{
		Connected: false,
		Context:   c.contextName,
	}

	version, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return status, fmt.Errorf("server version: %w", err)
	}
	status.ServerVersion = version.GitVersion

	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return status, fmt.Errorf("list nodes: %w", err)
	}

	status.NodeCount = len(nodes.Items)
	for _, node := range nodes.Items {
		for _, cond := range node.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				status.NodesReady++
				break
			}
		}
	}

	status.Connected = status.NodeCount > 0 && status.NodesReady == status.NodeCount
	return status, nil
}
