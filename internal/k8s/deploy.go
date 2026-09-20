package k8s

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type DeployParams struct {
	Namespace     string
	AppName       string
	Image         string
	ContainerPort int32
	LivenessPath  string
	ReadinessPath string
}

type WorkloadStatus struct {
	Namespace           string `json:"namespace"`
	DeploymentName      string `json:"deployment_name"`
	WorkloadKind        string `json:"workload_kind"`
	Image               string `json:"image"`
	ReplicasDesired     int32  `json:"replicas_desired"`
	ReplicasReady       int32  `json:"replicas_ready"`
	AvailableReplicas   int32  `json:"available_replicas"`
	DeploymentStatus    string `json:"deployment_status"`
}

func (c *Client) DeployApp(ctx context.Context, p DeployParams) (WorkloadStatus, error) {
	if p.ContainerPort == 0 {
		p.ContainerPort = 8080
	}
	if p.LivenessPath == "" {
		p.LivenessPath = "/health"
	}
	if p.ReadinessPath == "" {
		p.ReadinessPath = "/health"
	}

	labels := map[string]string{
		"app.kubernetes.io/name": p.AppName,
		"nimbus.io/managed-by":   "nimbus",
	}

	if err := c.ensureNamespace(ctx, p.Namespace, labels); err != nil {
		return WorkloadStatus{}, err
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.AppName,
			Namespace: p.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            p.AppName,
							Image:           p.Image,
							// Never: use images imported via `k3d image import` (no Docker Hub pull).
							// Iteration 8 (registry) will make this configurable.
							ImagePullPolicy: corev1.PullNever,
							Ports: []corev1.ContainerPort{
								{ContainerPort: p.ContainerPort, Name: "http"},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: p.LivenessPath,
										Port: intstrFromInt32(p.ContainerPort),
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: p.ReadinessPath,
										Port: intstrFromInt32(p.ContainerPort),
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				},
			},
		},
	}

	apps := c.clientset.AppsV1().Deployments(p.Namespace)
	existing, err := apps.Get(ctx, p.AppName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = apps.Create(ctx, deployment, metav1.CreateOptions{})
	} else if err == nil {
		deployment.ResourceVersion = existing.ResourceVersion
		_, err = apps.Update(ctx, deployment, metav1.UpdateOptions{})
	} else {
		return WorkloadStatus{}, fmt.Errorf("get deployment: %w", err)
	}
	if err != nil {
		return WorkloadStatus{}, fmt.Errorf("apply deployment: %w", err)
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.AppName,
			Namespace: p.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       p.ContainerPort,
					TargetPort: intstrFromInt32(p.ContainerPort),
				},
			},
		},
	}

	services := c.clientset.CoreV1().Services(p.Namespace)
	existingSvc, err := services.Get(ctx, p.AppName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = services.Create(ctx, svc, metav1.CreateOptions{})
	} else if err == nil {
		svc.ResourceVersion = existingSvc.ResourceVersion
		svc.Spec.ClusterIP = existingSvc.Spec.ClusterIP
		_, err = services.Update(ctx, svc, metav1.UpdateOptions{})
	} else {
		return WorkloadStatus{}, fmt.Errorf("get service: %w", err)
	}
	if err != nil {
		return WorkloadStatus{}, fmt.Errorf("apply service: %w", err)
	}

	return c.GetWorkloadStatus(ctx, p.Namespace, p.AppName)
}

func (c *Client) GetWorkloadStatus(ctx context.Context, namespace, appName string) (WorkloadStatus, error) {
	deployment, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, appName, metav1.GetOptions{})
	if err == nil {
		return workloadFromDeployment(namespace, deployment), nil
	}
	if !apierrors.IsNotFound(err) {
		return WorkloadStatus{}, fmt.Errorf("get deployment: %w", err)
	}

	sts, err := c.clientset.AppsV1().StatefulSets(namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return WorkloadStatus{}, fmt.Errorf("get deployment: %w", err)
		}
		return WorkloadStatus{}, fmt.Errorf("get statefulset: %w", err)
	}
	return workloadFromStatefulSet(namespace, sts), nil
}

func workloadFromDeployment(namespace string, deployment *appsv1.Deployment) WorkloadStatus {
	image := ""
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		image = deployment.Spec.Template.Spec.Containers[0].Image
	}
	return WorkloadStatus{
		Namespace:         namespace,
		DeploymentName:    deployment.Name,
		WorkloadKind:      "Deployment",
		Image:             image,
		ReplicasDesired:   derefInt32(deployment.Spec.Replicas),
		ReplicasReady:     deployment.Status.ReadyReplicas,
		AvailableReplicas: deployment.Status.AvailableReplicas,
		DeploymentStatus:  deploymentStatusFromDeployment(deployment),
	}
}

func workloadFromStatefulSet(namespace string, sts *appsv1.StatefulSet) WorkloadStatus {
	image := ""
	if len(sts.Spec.Template.Spec.Containers) > 0 {
		image = sts.Spec.Template.Spec.Containers[0].Image
	}
	return WorkloadStatus{
		Namespace:         namespace,
		DeploymentName:    sts.Name,
		WorkloadKind:      "StatefulSet",
		Image:             image,
		ReplicasDesired:   derefInt32(sts.Spec.Replicas),
		ReplicasReady:     sts.Status.ReadyReplicas,
		AvailableReplicas: sts.Status.ReadyReplicas,
		DeploymentStatus:  statefulSetStatus(sts),
	}
}

func (c *Client) EnsureNamespace(ctx context.Context, name string) error {
	labels := map[string]string{
		"nimbus.io/managed-by": "nimbus",
	}
	return c.ensureNamespace(ctx, name, labels)
}

func (c *Client) ensureNamespace(ctx context.Context, name string, labels map[string]string) error {
	nsClient := c.clientset.CoreV1().Namespaces()
	_, err := nsClient.Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get namespace: %w", err)
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
	_, err = nsClient.Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create namespace: %w", err)
	}
	return nil
}

func deploymentStatusFromDeployment(d *appsv1.Deployment) string {
	if d.Status.ReadyReplicas >= derefInt32(d.Spec.Replicas) && d.Status.ReadyReplicas > 0 {
		return "running"
	}
	if d.Status.UnavailableReplicas > 0 || d.Status.Replicas > d.Status.ReadyReplicas {
		return "progressing"
	}
	if d.Status.Replicas == 0 {
		return "pending"
	}
	return "unknown"
}

func statefulSetStatus(sts *appsv1.StatefulSet) string {
	desired := derefInt32(sts.Spec.Replicas)
	if sts.Status.ReadyReplicas >= desired && sts.Status.ReadyReplicas > 0 {
		return "running"
	}
	if sts.Status.Replicas > sts.Status.ReadyReplicas {
		return "progressing"
	}
	if sts.Status.Replicas == 0 {
		return "pending"
	}
	return "unknown"
}

func int32Ptr(v int32) *int32 { return &v }

func derefInt32(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func intstrFromInt32(port int32) intstr.IntOrString {
	return intstr.FromInt32(port)
}