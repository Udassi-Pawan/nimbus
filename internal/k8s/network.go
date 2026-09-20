package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

type ServiceNetworkInfo struct {
	Namespace        string `json:"namespace"`
	ServiceName      string `json:"service_name"`
	Port             int32  `json:"port"`
	ClusterIP        string `json:"cluster_ip"`
	DNSShort         string `json:"dns_short"`
	DNSFQDN          string `json:"dns_fqdn"`
	IngressHost      string `json:"ingress_host,omitempty"`
	IngressURL       string `json:"ingress_url,omitempty"`
	EndpointsReady   int    `json:"endpoints_ready"`
	ServiceFound     bool   `json:"service_found"`
}

type ConnectivityResult struct {
	OK                bool   `json:"ok"`
	Message           string `json:"message"`
	SourceNamespace   string `json:"source_namespace"`
	TargetNamespace   string `json:"target_namespace"`
	TargetServiceName string `json:"target_service_name"`
	TargetURL         string `json:"target_url"`
	EndpointsReady    int    `json:"endpoints_ready"`
}

func (c *Client) GetServiceNetwork(ctx context.Context, namespace, serviceName string) (ServiceNetworkInfo, error) {
	info := ServiceNetworkInfo{
		Namespace:   namespace,
		ServiceName: serviceName,
		Port:        8080,
	}

	svc, err := c.clientset.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return info, nil
		}
		return info, fmt.Errorf("get service: %w", err)
	}
	info.ServiceFound = true
	info.ClusterIP = svc.Spec.ClusterIP
	if len(svc.Spec.Ports) > 0 {
		info.Port = svc.Spec.Ports[0].Port
	}

	info.DNSShort = fmt.Sprintf("http://%s:%d", serviceName, info.Port)
	info.DNSFQDN = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", serviceName, namespace, info.Port)

	info.EndpointsReady = countReadyEndpoints(ctx, c, namespace, serviceName)

	ingress, err := c.clientset.NetworkingV1().Ingresses(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err == nil && len(ingress.Spec.Rules) > 0 {
		host := ingress.Spec.Rules[0].Host
		if host != "" {
			info.IngressHost = host
			info.IngressURL = fmt.Sprintf("http://%s", host)
		}
	}

	return info, nil
}

func (c *Client) CheckConnectivity(ctx context.Context, sourceNamespace, targetNamespace, targetServiceName, healthPath string) (ConnectivityResult, error) {
	if healthPath == "" {
		healthPath = "/health"
	}
	if !strings.HasPrefix(healthPath, "/") {
		healthPath = "/" + healthPath
	}

	net, err := c.GetServiceNetwork(ctx, targetNamespace, targetServiceName)
	if err != nil {
		return ConnectivityResult{}, err
	}

	targetURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d%s", targetServiceName, targetNamespace, net.Port, healthPath)

	result := ConnectivityResult{
		SourceNamespace:   sourceNamespace,
		TargetNamespace:   targetNamespace,
		TargetServiceName: targetServiceName,
		TargetURL:         targetURL,
		EndpointsReady:    net.EndpointsReady,
	}

	if !net.ServiceFound {
		result.Message = "target Service not found in cluster (deploy target first)"
		return result, nil
	}
	if net.EndpointsReady == 0 {
		result.Message = "target Service has no ready endpoints (pod not ready or missing)"
		return result, nil
	}

	if err := c.runCurlProbe(ctx, sourceNamespace, targetURL); err != nil {
		result.Message = err.Error()
		return result, nil
	}

	result.OK = true
	result.Message = "connectivity check passed (curl from source namespace)"
	return result, nil
}

func countReadyEndpoints(ctx context.Context, c *Client, namespace, serviceName string) int {
	ep, err := c.clientset.CoreV1().Endpoints(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return 0
	}
	total := 0
	for _, subset := range ep.Subsets {
		total += len(subset.Addresses)
	}
	return total
}

func (c *Client) runCurlProbe(ctx context.Context, namespace, url string) error {
	jobName := fmt.Sprintf("nimbus-curl-%d", time.Now().UnixNano())
	labels := map[string]string{
		"app.kubernetes.io/name":     "nimbus-connectivity",
		"nimbus.io/managed-by":       "nimbus",
		"nimbus.io/connectivity-job": "true",
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32Ptr(0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "curl",
							Image:   "curlimages/curl:8.5.0",
							Command: []string{"curl", "-sf", "--max-time", "5", url},
						},
					},
				},
			},
		},
	}

	jobs := c.clientset.BatchV1().Jobs(namespace)
	created, err := jobs.Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create curl job: %w", err)
	}
	defer func() {
		propagation := metav1.DeletePropagationBackground
		_ = jobs.Delete(context.Background(), created.Name, metav1.DeleteOptions{PropagationPolicy: &propagation})
	}()

	waitCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	var jobErr error
	err = wait.PollUntilContextCancel(waitCtx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		j, err := jobs.Get(ctx, created.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, cond := range j.Status.Conditions {
			if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
				return true, nil
			}
			if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
				jobErr = fmt.Errorf("curl probe failed (no route, network policy, or app error)")
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("wait for curl job: %w", err)
	}
	if jobErr != nil {
		return jobErr
	}
	return nil
}
