package k8s

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PVCNameForStatefulSet matches volumeClaimTemplates named "data" (pod 0).
func PVCNameForStatefulSet(statefulSetName string) string {
	return fmt.Sprintf("data-%s-0", statefulSetName)
}

func (c *Client) GetPVCPhase(ctx context.Context, namespace, pvcName string) (string, error) {
	pvc, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("get pvc: %w", err)
	}
	return string(pvc.Status.Phase), nil
}
