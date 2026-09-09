//go:build e2e
// +build e2e

/*
Copyright 2025 Thomas Boerger <thomas@webhippie.de>.

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

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/kubehippie/external-dns-watcher/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	// projectImage is the name of the image which will be build and loaded
	// with the code source changes to be tested.
	projectImage = "ghcr.io/kubehippie/external-dns-watcher:local"

	// externalDNSVersion is the external-dns Helm chart version used for the
	// test instance. Override via the EXTERNAL_DNS_VERSION environment
	// variable. Empty means the latest chart version is installed.
	externalDNSVersion = envOrDefault("EXTERNAL_DNS_VERSION", "0.21.0")

	// skipExternalDNSInstall disables the external-dns installation during
	// test setup when EXTERNAL_DNS_INSTALL_SKIP=true. Useful if external-dns
	// is already installed in the cluster.
	skipExternalDNSInstall = os.Getenv("EXTERNAL_DNS_INSTALL_SKIP") == "true"

	// keepExternalDNSInstall keeps the installation of external-dns after the
	// execution of the test suite when EXTERNAL_DNS_KEEP_INSTALL=true.
	keepExternalDNSInstall = os.Getenv("EXTERNAL_DNS_KEEP_INSTALL") == "true"

	// isExternalDNSAlreadyInstalled is true when the DNSEndpoint CRD is found
	// on the cluster.
	isExternalDNSAlreadyInstalled = false

	// certManagerVersion is the CertManager version tag used for the test
	// instance. Override via the CERT_MANAGER_VERSION environment variable.
	certManagerVersion = envOrDefault("CERT_MANAGER_VERSION", "1.20.2")

	// skipCertManagerInstall disables the Cert Manager reconciliation suite
	// when CERT_MANAGER_INSTALL_SKIP=true.
	skipCertManagerInstall = os.Getenv("CERT_MANAGER_INSTALL_SKIP") == "true"

	// keepCertManagerInstall keeps the installation of Cert Manager after the
	// execution of the test suite when CERT_MANAGER_KEEP_INSTALL=true.
	keepCertManagerInstall = os.Getenv("CERT_MANAGER_KEEP_INSTALL") == "true"

	// isCertManagerAlreadyInstalled is true when the Cert Manager namespace
	// already exists.
	isCertManagerAlreadyInstalled = false

	// ci is true when the CI environment variable is set, indicating the
	// tests are running on an ephemeral CI runner where cleanup is not
	// necessary.
	ci = os.Getenv("CI") == "true"
)

// TestE2E runs the end-to-end (e2e) test suite for the project. These tests execute in an isolated,
// temporary environment to validate project changes with the purpose of being used in CI jobs.
// The default setup requires Kind, builds/loads the Manager Docker image locally, and installs
// CertManager.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting external-dns-watcher integration test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	By("building the manager image")
	cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectImage))
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the manager image")

	By("loading the image on Kind")
	err = utils.LoadImageToKindClusterWithName(projectImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the image on Kind")

	if !skipCertManagerInstall {
		By("checking if CertManager is installed already")
		isCertManagerAlreadyInstalled = utils.IsCertManagerInstalled()
		if !isCertManagerAlreadyInstalled {
			_, _ = fmt.Fprintf(GinkgoWriter, "Installing CertManager (%s)...\n", certManagerVersion)
			Expect(utils.InstallCertManager(certManagerVersion)).To(Succeed(), "Failed to install CertManager")
		} else {
			_, _ = fmt.Fprintf(GinkgoWriter, "WARNING: CertManager is already installed. Skipping installation...\n")
		}
	}

	if !skipExternalDNSInstall {
		By("checking if ExternalDNS is installed already")
		isExternalDNSAlreadyInstalled = utils.IsExternalDNSInstalled()
		if !isExternalDNSAlreadyInstalled {
			_, _ = fmt.Fprintf(GinkgoWriter, "Installing ExternalDNS (%s)...\n", externalDNSVersion)
			Expect(utils.InstallExternalDNS(externalDNSVersion)).To(Succeed(), "Failed to install ExternalDNS")
		} else {
			_, _ = fmt.Fprintf(GinkgoWriter, "WARNING: ExternalDNS is already installed. Skipping installation...\n")
		}
	}

	By("creating manager namespace")
	cmd = exec.Command("kubectl", "create", "ns", namespace)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

	By("labeling the security policy")
	cmd = exec.Command("kubectl", "label", "--overwrite",
		"ns", namespace,
		"pod-security.kubernetes.io/enforce=restricted",
	)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to label the security policy")

	By("installing manager CRDs")
	cmd = exec.Command("make", "install")
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to install manager CRDs")

	By("deploying the manager")
	cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy manager")

	By("waiting for the serving certificate to be issued by cert-manager")
	cmd = exec.Command("kubectl", "wait",
		"--for=condition=Ready",
		"certificate/external-dns-watcher-serving-cert",
		"-n", namespace,
		"--timeout=2m",
	)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Serving certificate was not issued in time")

	By("waiting for manager deployment to be ready")
	cmd = exec.Command("kubectl", "rollout", "status",
		"deployment/external-dns-watcher-controller-manager",
		"-n", namespace,
		"--timeout=2m",
	)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Manager deployment did not become ready in time")
})

var _ = AfterSuite(func() {
	if ci {
		return
	}

	By("undeploying the manager")
	cmd := exec.Command("make", "undeploy")
	_, _ = utils.Run(cmd)

	By("uninstalling manager CRDs")
	cmd = exec.Command("make", "uninstall")
	_, _ = utils.Run(cmd)

	By("removing manager namespace")
	cmd = exec.Command("kubectl", "delete", "ns", namespace)
	_, _ = utils.Run(cmd)

	if !skipCertManagerInstall && !isCertManagerAlreadyInstalled && !keepCertManagerInstall {
		_, _ = fmt.Fprintf(GinkgoWriter, "Uninstalling CertManager...\n")
		utils.UninstallCertManager()
	}

	if !skipExternalDNSInstall && !isExternalDNSAlreadyInstalled && !keepExternalDNSInstall {
		_, _ = fmt.Fprintf(GinkgoWriter, "Uninstalling ExternalDNS...\n")
		utils.UninstallExternalDNS()
	}
})

// envOrDefault returns the value of the named environment variable, or
// defaultVal when the variable is unset or empty.
func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
