package k8s

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type HelmDeployParams struct {
	ReleaseName   string
	ChartPath     string
	Namespace     string
	Image         string
	PullPolicy    string
}

// DeployHelm runs `helm upgrade --install` against a chart on disk (generated/{slug}/helm).
func (c *Client) DeployHelm(ctx context.Context, p HelmDeployParams) error {
	helmBin, err := exec.LookPath("helm")
	if err != nil {
		return fmt.Errorf("helm not found on PATH: install Helm 3 (https://helm.sh/docs/intro/install/)")
	}

	if _, err := os.Stat(p.ChartPath); err != nil {
		return fmt.Errorf("chart path %s: %w", p.ChartPath, err)
	}

	repo, tag := parseImageRef(p.Image)
	if tag == "" {
		tag = "latest"
	}
	pullPolicy := p.PullPolicy
	if pullPolicy == "" {
		pullPolicy = "Never"
	}

	args := []string{
		"upgrade", "--install", p.ReleaseName, p.ChartPath,
		"--namespace", p.Namespace,
		"--create-namespace",
		"--atomic",
		"--wait",
		"--timeout", "3m",
		"--set", fmt.Sprintf("image.repository=%s", repo),
		"--set", fmt.Sprintf("image.tag=%s", tag),
		"--set", fmt.Sprintf("image.pullPolicy=%s", pullPolicy),
	}

	if c.contextName != "" {
		args = append(args, "--kube-context", c.contextName)
	}

	cmd := exec.CommandContext(ctx, helmBin, args...)
	cmd.Env = os.Environ()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("helm upgrade --install: %s", msg)
	}
	return nil
}
