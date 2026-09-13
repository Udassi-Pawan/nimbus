package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Udassi-Pawan/nimbus/internal/paths"
	"github.com/Udassi-Pawan/nimbus/internal/templates"
)

func main() {
	wd, _ := os.Getwd()
	fmt.Println("cwd:", wd)

	repoRoot, err := paths.FindRepoRoot()
	if err != nil {
		fmt.Println("ERR:", err)
		os.Exit(1)
	}

	_, err = templates.GenerateGoAPI(
		paths.ResolvePath(repoRoot, "generated"),
		filepath.Join(repoRoot, "templates", "go-api"),
		templates.ServiceTemplateInput{
		Name:          "Test",
		Slug:          "test-api",
		Description:   "d",
		RepositoryURL: "http://x",
		OwnerEmail:    "a@b.c",
		},
	)
	if err != nil {
		fmt.Println("ERR:", err)
		os.Exit(1)
	}
	fmt.Println("OK")
}
