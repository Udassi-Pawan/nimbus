package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

type ServiceTemplateInput struct {
	Name          string
	Slug          string
	Description   string
	RepositoryURL string
	OwnerEmail    string
}

type GenerateResult struct {
	OutputPath string   `json:"output_path"`
	Files      []string `json:"files"`
	TemplateID string   `json:"template_id"`
}

type fileMapping struct {
	TemplatePath string
	OutputPath   string
}

func WorkloadTypeForTemplate(templateID string) string {
	switch templateID {
	case "postgres", "redis":
		return "stateful"
	default:
		return "stateless"
	}
}

func Generate(templateID, baseOutputDir, templateDir string, input ServiceTemplateInput) (GenerateResult, error) {
	switch templateID {
	case "go-api":
		return generateWithMappings(templateID, baseOutputDir, templateDir, input, goAPIMappings())
	case "postgres":
		return generateWithMappings(templateID, baseOutputDir, templateDir, input, postgresMappings())
	case "redis":
		return generateWithMappings(templateID, baseOutputDir, templateDir, input, redisMappings())
	default:
		return GenerateResult{}, fmt.Errorf("unsupported template_id %q", templateID)
	}
}

func GenerateGoAPI(baseOutputDir, templateDir string, input ServiceTemplateInput) (GenerateResult, error) {
	return Generate("go-api", baseOutputDir, templateDir, input)
}

func generateWithMappings(templateID, baseOutputDir, templateDir string, input ServiceTemplateInput, mappings []fileMapping) (GenerateResult, error) {
	outputDir := filepath.Join(baseOutputDir, input.Slug)

	if _, err := os.Stat(templateDir); err != nil {
		return GenerateResult{}, fmt.Errorf("template dir %s: %w", templateDir, err)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return GenerateResult{}, fmt.Errorf("create output dir: %w", err)
	}

	var written []string

	for _, m := range mappings {
		tmplPath := filepath.Join(templateDir, m.TemplatePath)
		outPath := filepath.Join(outputDir, m.OutputPath)

		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return GenerateResult{}, err
		}

		tmpl, err := template.ParseFiles(tmplPath)
		if err != nil {
			return GenerateResult{}, fmt.Errorf("parse template %s: %w", m.TemplatePath, err)
		}

		f, err := os.Create(outPath)
		if err != nil {
			return GenerateResult{}, fmt.Errorf("create file %s: %w", outPath, err)
		}

		if err := tmpl.Execute(f, input); err != nil {
			f.Close()
			return GenerateResult{}, fmt.Errorf("execute template %s: %w", m.TemplatePath, err)
		}
		f.Close()
		written = append(written, m.OutputPath)
	}

	return GenerateResult{
		OutputPath: outputDir,
		Files:      written,
		TemplateID: templateID,
	}, nil
}

func goAPIMappings() []fileMapping {
	return []fileMapping{
		{"cmd_server_main.go.tmpl", "cmd/server/main.go"},
		{"Dockerfile.tmpl", "Dockerfile"},
		{"README.md.tmpl", "README.md"},
		{"helm/Chart.yaml.tmpl", "helm/Chart.yaml"},
		{"helm/values.yaml.tmpl", "helm/values.yaml"},
		{"helm/deployment.yaml.tmpl", "helm/templates/deployment.yaml"},
		{"helm/service.yaml.tmpl", "helm/templates/service.yaml"},
		{"helm/ingress.yaml.tmpl", "helm/templates/ingress.yaml"},
		{"kustomize/base/kustomization.yaml.tmpl", "kustomize/base/kustomization.yaml"},
		{"kustomize/overlays/dev/kustomization.yaml.tmpl", "kustomize/overlays/dev/kustomization.yaml"},
		{"github/ci.yml.tmpl", ".github/workflows/ci.yml"},
	}
}

func postgresMappings() []fileMapping {
	return []fileMapping{
		{"README.md.tmpl", "README.md"},
		{"helm/Chart.yaml.tmpl", "helm/Chart.yaml"},
		{"helm/values.yaml.tmpl", "helm/values.yaml"},
		{"helm/statefulset.yaml.tmpl", "helm/templates/statefulset.yaml"},
		{"helm/service.yaml.tmpl", "helm/templates/service.yaml"},
	}
}

func redisMappings() []fileMapping {
	return []fileMapping{
		{"README.md.tmpl", "README.md"},
		{"helm/Chart.yaml.tmpl", "helm/Chart.yaml"},
		{"helm/values.yaml.tmpl", "helm/values.yaml"},
		{"helm/statefulset.yaml.tmpl", "helm/templates/statefulset.yaml"},
		{"helm/service.yaml.tmpl", "helm/templates/service.yaml"},
	}
}
