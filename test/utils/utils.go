/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2" // nolint:revive,staticcheck
)

const (
	defaultKindBinary  = "kind"
	defaultKindCluster = "kind"
)

// normalizeImageTag ensures the given version string is prefixed with "v", as
// required by the external-dns container image tags published to
// registry.k8s.io (e.g. "v0.22.0"), regardless of whether the caller passed
// the version with or without the prefix.
func normalizeImageTag(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

// InstallExternalDNS installs external-dns via Helm using the in-memory provider,
// which requires no cloud credentials and is suitable for e2e testing.
func InstallExternalDNS(version string) error {
	projectDir, err := GetProjectDir()
	if err != nil {
		return err
	}

	valuesFile := filepath.Join(
		projectDir,
		"test",
		"e2e",
		"testdata",
		"external-dns.yaml",
	)

	cmd := exec.Command("helm", "upgrade", "--install", "external-dns",
		"external-dns",
		"--repo", "https://kubernetes-sigs.github.io/external-dns",
		"--create-namespace",
		"--namespace", "external-dns",
		"--values", valuesFile,
		"--set", "image.tag="+normalizeImageTag(version),
		"--timeout", "5m",
		"--wait",
		"--hide-notes",
	)
	if _, err := Run(cmd); err != nil {
		return err
	}
	return nil
}

// UninstallExternalDNS removes the external-dns Helm release and its namespace.
func UninstallExternalDNS() {
	cmd := exec.Command("helm", "uninstall", "external-dns",
		"--namespace", "external-dns",
		"--ignore-not-found",
	)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}

	cmd = exec.Command("kubectl", "delete", "namespace",
		"external-dns",
		"--ignore-not-found",
	)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// IsExternalDNSInstalled returns true when the external-dns Helm release
// already exists.
func IsExternalDNSInstalled() bool {
	cmd := exec.Command("helm", "status", "external-dns",
		"--namespace", "external-dns",
	)
	_, err := Run(cmd)
	return err == nil
}

// InstallCertManager deploys CertManager into the Kind cluster using the
// official cert-manager Helm chart at the given version tag. It blocks until
// the Deployment is ready (up to 5 minutes).
func InstallCertManager(version string) error {
	projectDir, err := GetProjectDir()
	if err != nil {
		return err
	}

	valuesFile := filepath.Join(
		projectDir,
		"test",
		"e2e",
		"testdata",
		"cert-manager.yaml",
	)

	cmd := exec.Command("helm", "upgrade", "--install", "cert-manager",
		"oci://quay.io/jetstack/charts/cert-manager",
		"--create-namespace",
		"--namespace", "cert-manager",
		"--values", valuesFile,
		"--version", version,
		"--timeout", "5m",
		"--wait",
		"--hide-notes",
	)
	if _, err := Run(cmd); err != nil {
		return err
	}
	return nil
}

// UninstallCertManager removes the cert-manager Helm release and its namespace.
func UninstallCertManager() {
	cmd := exec.Command("helm", "uninstall", "cert-manager",
		"--namespace", "cert-manager",
		"--ignore-not-found",
	)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}

	cmd = exec.Command("kubectl", "delete", "namespace",
		"cert-manager",
		"--ignore-not-found",
	)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// IsCertManagerInstalled returns true when the cert-manager Helm release
// already exists.
func IsCertManagerInstalled() bool {
	cmd := exec.Command("helm", "status", "cert-manager",
		"--namespace", "cert-manager",
	)
	_, err := Run(cmd)
	return err == nil
}

func warnError(err error) {
	_, _ = fmt.Fprintf(GinkgoWriter, "warning: %v\n", err)
}

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) (string, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "chdir dir: %q\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	_, _ = fmt.Fprintf(GinkgoWriter, "running: %q\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%q failed with error %q: %w", command, string(output), err)
	}

	return string(output), nil
}

func LoadImageToKindClusterWithName(name string) error {
	cluster := defaultKindCluster
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}
	kindBinary := defaultKindBinary
	if v, ok := os.LookupEnv("KIND"); ok {
		kindBinary = v
	}
	cmd := exec.Command(kindBinary, kindOptions...)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.Split(output, "\n")
	for _, element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, fmt.Errorf("failed to get current working directory: %w", err)
	}
	wd = strings.ReplaceAll(wd, "/test/e2e", "")
	return wd, nil
}

// UncommentCode searches for target in the file and remove the comment prefix
// of the target content. The target content may span multiple lines.
func UncommentCode(filename, target, prefix string) error {
	// false positive
	// nolint:gosec
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %q: %w", filename, err)
	}
	strContent := string(content)

	idx := strings.Index(strContent, target)
	if idx < 0 {
		return fmt.Errorf("unable to find the code %q to be uncomment", target)
	}

	out := new(bytes.Buffer)
	_, err = out.Write(content[:idx])
	if err != nil {
		return fmt.Errorf("failed to write to output: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewBufferString(target))
	if !scanner.Scan() {
		return nil
	}
	for {
		if _, err = out.WriteString(strings.TrimPrefix(scanner.Text(), prefix)); err != nil {
			return fmt.Errorf("failed to write to output: %w", err)
		}
		// Avoid writing a newline in case the previous line was the last in target.
		if !scanner.Scan() {
			break
		}
		if _, err = out.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write to output: %w", err)
		}
	}

	if _, err = out.Write(content[idx+len(target):]); err != nil {
		return fmt.Errorf("failed to write to output: %w", err)
	}

	// false positive
	// nolint:gosec
	if err = os.WriteFile(filename, out.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write file %q: %w", filename, err)
	}

	return nil
}
