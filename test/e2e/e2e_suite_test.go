//go:build e2e

/*
Copyright 2026 Bohdan Leshchenko.

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
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"
	ctrl "sigs.k8s.io/controller-runtime"

	infrav1 "github.com/thebhdn/cluster-api-provider-libvirt/api/v1alpha1"
)

const (
	bootstrapClusterName = "bootstrap"
	kindIPFamily         = "IPv4"
	ginkgoLogFile        = "ginkgo-log.txt"
	repositoryFolderName = "repository"
)

var (
	configPath         string
	artifactFolder     string
	useExistingCluster bool
	skipCleanup        bool
)

var (
	ctx                       = ctrl.SetupSignalHandler()
	watchesCtx, cancelWatches = context.WithCancel(ctx)

	e2eConfig                *clusterctl.E2EConfig
	clusterctlConfigPath     string
	bootstrapClusterProvider bootstrap.ClusterProvider
	bootstrapClusterProxy    framework.ClusterProxy
)

func init() {
	flag.StringVar(&configPath, "e2e.config", "", "path to the e2e config file")
	flag.StringVar(&artifactFolder, "e2e.artifacts-folder", "", "folder for test artifacts (logs, resource dumps)")
	flag.BoolVar(&useExistingCluster, "e2e.use-existing-cluster", false,
		"use the current kubeconfig cluster instead of creating a kind cluster")
	flag.BoolVar(&skipCleanup, "e2e.skip-resource-cleanup", false,
		"keep the workload cluster and the kind cluster after the tests")
}

func TestE2E(t *testing.T) {
	g := NewWithT(t)
	ctrl.SetLogger(GinkgoLogr)

	g.Expect(os.MkdirAll(artifactFolder, 0o750)).To(Succeed(), "cannot create e2e.artifacts-folder %q", artifactFolder)

	RegisterFailHandler(Fail)

	w, err := ginkgoextensions.EnableFileLogging(filepath.Join(artifactFolder, ginkgoLogFile))
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = w.Close() }()

	RunSpecs(t, "caplv-e2e")
}

var _ = BeforeSuite(func() {
	Expect(configPath).To(BeAnExistingFile(), "e2e.config must be an existing file")

	By("Loading the e2e config")
	e2eConfig = clusterctl.LoadE2EConfig(ctx, clusterctl.LoadE2EConfigInput{ConfigPath: configPath})
	Expect(e2eConfig).ToNot(BeNil(), "failed to load the e2e config from %s", configPath)
	for _, name := range requiredVariables {
		Expect(e2eConfig.GetVariableOrEmpty(name)).ToNot(BeEmpty(), "set the %s env var, see docs/development.md", name)
	}

	By("Creating a local clusterctl repository")
	clusterctlConfigPath = createClusterctlRepository(filepath.Join(artifactFolder, repositoryFolderName))

	By("Setting up the bootstrap cluster")
	bootstrapClusterProvider, bootstrapClusterProxy = setupBootstrapCluster()

	By("Installing the providers")
	initInput := clusterctl.InitManagementClusterAndWatchControllerLogsInput{
		ClusterProxy:            bootstrapClusterProxy,
		ClusterctlConfigPath:    clusterctlConfigPath,
		InfrastructureProviders: e2eConfig.InfrastructureProviders(),
		LogFolder:               filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
	}
	clusterctl.InitManagementClusterAndWatchControllerLogs(watchesCtx, initInput,
		e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
})

var _ = AfterSuite(func() {
	if skipCleanup {
		return
	}
	By("Tearing down the bootstrap cluster")
	cancelWatches()
	if bootstrapClusterProxy != nil {
		bootstrapClusterProxy.Dispose(ctx)
	}
	if bootstrapClusterProvider != nil {
		bootstrapClusterProvider.Dispose(ctx)
	}
})

func initScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	framework.TryAddDefaultSchemes(scheme)
	Expect(infrav1.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func createClusterctlRepository(folder string) string {
	input := clusterctl.CreateRepositoryInput{
		E2EConfig:        e2eConfig,
		RepositoryFolder: folder,
	}
	cniPath := e2eConfig.MustGetVariable(capie2e.CNIPath)
	Expect(cniPath).To(BeAnExistingFile(), "the %s variable must point to an existing file", capie2e.CNIPath)
	input.RegisterClusterResourceSetConfigMapTransformation(cniPath, capie2e.CNIResources)

	path := clusterctl.CreateRepository(ctx, input)
	Expect(path).To(BeAnExistingFile(), "the clusterctl config is missing in %s", folder)
	return path
}

func setupBootstrapCluster() (bootstrap.ClusterProvider, framework.ClusterProxy) {
	var provider bootstrap.ClusterProvider
	kubeconfigPath := ""
	if !useExistingCluster {
		kindInput := bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
			Name:              e2eConfig.ManagementClusterName,
			KubernetesVersion: e2eConfig.MustGetVariable(capie2e.KubernetesVersionManagement),
			Images:            e2eConfig.Images,
			IPFamily:          kindIPFamily,
			LogFolder:         filepath.Join(artifactFolder, "kind"),
		}
		provider = bootstrap.CreateKindBootstrapClusterAndLoadImages(ctx, kindInput)
		Expect(provider).ToNot(BeNil(), "failed to create the kind bootstrap cluster")
		kubeconfigPath = provider.GetKubeconfigPath()
	}

	proxy := framework.NewClusterProxy(bootstrapClusterName, kubeconfigPath, initScheme())
	Expect(proxy).ToNot(BeNil(), "failed to create the bootstrap cluster proxy")
	return provider, proxy
}
