package k8s

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GeneratePassword(length int) (string, error) {
	if length <= 0 {
		length = 24
	}
	buf := make([]byte, (length+1)/2)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	out := hex.EncodeToString(buf)
	if len(out) > length {
		out = out[:length]
	}
	return out, nil
}

// EnsureOpaqueSecret creates or updates a generic secret (does not return secret values via API).
func (c *Client) SecretExists(ctx context.Context, namespace, name string) (bool, error) {
	_, err := c.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return true, nil
	}
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("get secret: %w", err)
}

func (c *Client) EnsureOpaqueSecret(ctx context.Context, namespace, name string, data map[string]string) error {
	exists, err := c.SecretExists(ctx, namespace, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	byteData := map[string][]byte{}
	for k, v := range data {
		byteData[k] = []byte(v)
	}

	_, err = c.clientset.CoreV1().Secrets(namespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"nimbus.io/managed-by": "nimbus",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: byteData,
	}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create secret: %w", err)
	}
	return nil
}

// CopySecretToNamespace copies a secret by name from srcNS into dstNS (required for secretKeyRef in app pods).
func (c *Client) CopySecretToNamespace(ctx context.Context, srcNS, dstNS, name string) error {
	if srcNS == dstNS {
		return nil
	}
	if err := c.EnsureNamespace(ctx, dstNS); err != nil {
		return err
	}

	src, err := c.clientset.CoreV1().Secrets(srcNS).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("secret %q not found in %s (deploy the dependency in the same environment first)", name, srcNS)
		}
		return fmt.Errorf("get source secret: %w", err)
	}

	dest := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: dstNS,
			Labels: map[string]string{
				"nimbus.io/managed-by":       "nimbus",
				"nimbus.io/copied-from-ns": srcNS,
			},
		},
		Type: src.Type,
		Data: src.Data,
	}

	secrets := c.clientset.CoreV1().Secrets(dstNS)
	existing, err := secrets.Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = secrets.Create(ctx, dest, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create secret in %s: %w", dstNS, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get dest secret: %w", err)
	}

	dest.ResourceVersion = existing.ResourceVersion
	_, err = secrets.Update(ctx, dest, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update secret in %s: %w", dstNS, err)
	}
	return nil
}
