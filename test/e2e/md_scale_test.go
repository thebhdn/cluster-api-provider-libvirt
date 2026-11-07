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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
)

var _ = Describe("When scaling a MachineDeployment", func() {
	var specNamespace string

	// Outer AfterEach runs after the spec's own cleanup, which deletes the cluster.
	AfterEach(func() {
		if skipCleanup || specNamespace == "" {
			return
		}
		By("Checking that nothing is left on the libvirt host")
		leftovers, err := findHostLeftovers(specNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(leftovers.empty()).To(BeTrue(), "left on the libvirt host after delete: %+v", leftovers)
	})

	Context("with 1 control plane and 1 to 3 workers", func() {
		capie2e.MachineDeploymentScaleSpec(ctx, func() capie2e.MachineDeploymentScaleSpecInput {
			return capie2e.MachineDeploymentScaleSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				PostNamespaceCreated: func(_ framework.ClusterProxy, namespace string) {
					specNamespace = namespace
				},
			}
		})
	})
})
