// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package kubernetes

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	api "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccKubernetesPod_minimal(t *testing.T) {
	podNname := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podNname)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigMinimal(podNname, busyboxImageVersion),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.generation"),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.resource_version"),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.uid"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
			{
				Config:   testAccKubernetesPodConfigMinimal(podNname, busyboxImageVersion),
				PlanOnly: true,
			},
		},
	})
}

func TestAccKubernetesPod_basic(t *testing.T) {
	var conf1 api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	secretName := acctest.RandomWithPrefix("tf-acc-test")
	configMapName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	image := alpineImage

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigBasic(secretName, configMapName, podName, image),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf1),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.annotations.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.labels.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.labels.app", "pod_label"),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.name", podName),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.generation"),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.resource_version"),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.uid"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env.0.value_from.0.secret_key_ref.0.name", secretName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env.0.value_from.0.secret_key_ref.0.key", "one"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env.0.value_from.0.secret_key_ref.0.optional", "true"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env.1.value_from.0.config_map_key_ref.0.name", configMapName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env.1.value_from.0.config_map_key_ref.0.key", "one"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env.1.value_from.0.config_map_key_ref.0.optional", "true"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env_from.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env_from.0.config_map_ref.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env_from.0.config_map_ref.0.name", fmt.Sprintf("%s-from", configMapName)),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env_from.0.config_map_ref.0.optional", "true"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env_from.0.prefix", "FROM_CM_"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env_from.1.secret_ref.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env_from.1.secret_ref.0.name", fmt.Sprintf("%s-from", secretName)),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env_from.1.secret_ref.0.optional", "false"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env_from.1.prefix", "FROM_S_"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", image),
					resource.TestCheckResourceAttr(resourceName, "spec.0.topology_spread_constraint.#", "0"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_scheduler(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	schedulerName := acctest.RandomWithPrefix("test-scheduler")
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod_v1.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipIfClusterVersionLessThan(t, "1.22.0")
			setClusterVersionVar(t, "TF_VAR_scheduler_cluster_version") // should be in format 'vX.Y.Z'
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesCustomScheduler(schedulerName),
			},
			{
				Config: testAccKubernetesCustomScheduler(schedulerName) +
					testAccKubernetesPodConfigScheduler(podName, schedulerName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.name", podName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.scheduler_name", schedulerName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
				),
			},
		},
	})
}

func TestAccKubernetesPod_initContainer_updateForcesNew(t *testing.T) {
	var conf1, conf2 api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	image := busyboxImageVersion
	image1 := busyboxImageVersion1
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithInitContainer(podName, image),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf1),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.name", podName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.name", "container"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.init_container.0.name", "initcontainer"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.init_container.0.image", image),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
			{
				Config: testAccKubernetesPodConfigWithInitContainer(podName, image1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf2),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.name", podName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.name", "container"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.init_container.0.name", "initcontainer"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.init_container.0.image", image1),
					testAccCheckKubernetesPodForceNew(&conf1, &conf2, true),
				),
			},
		},
	})
}

func TestAccKubernetesPod_updateArgsForceNew(t *testing.T) {
	var conf1 api.Pod
	var conf2 api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")

	imageName := "hashicorp/http-echo:latest"
	argsBefore := `["-listen=:80", "-text='before modification'"]`
	argsAfter := `["-listen=:80", "-text='after modification'"]`
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigArgsUpdate(podName, imageName, argsBefore),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf1),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.annotations.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.name", podName),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.generation"),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.resource_version"),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.uid"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.args.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.args.0", "-listen=:80"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.args.1", "-text='before modification'"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.name", "containername"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
			{
				Config: testAccKubernetesPodConfigArgsUpdate(podName, imageName, argsAfter),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf2),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.annotations.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.name", podName),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.generation"),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.resource_version"),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.uid"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.args.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.args.0", "-listen=:80"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.args.1", "-text='after modification'"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.name", "containername"),
					testAccCheckKubernetesPodForceNew(&conf1, &conf2, true),
				),
			},
		},
	})
}

func TestAccKubernetesPod_updateEnvForceNew(t *testing.T) {
	var conf1 api.Pod
	var conf2 api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")

	imageName := "hashicorp/http-echo:latest"
	envBefore := "bar"
	envAfter := "baz"
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigEnvUpdate(podName, imageName, envBefore),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf1),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.annotations.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.name", podName),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.generation"),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.resource_version"),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.uid"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env.0.name", "foo"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env.0.value", "bar"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.name", "containername"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
			{
				Config: testAccKubernetesPodConfigEnvUpdate(podName, imageName, envAfter),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf2),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.annotations.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.name", podName),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.generation"),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.resource_version"),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.uid"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env.0.name", "foo"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.env.0.value", "baz"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.name", "containername"),
					testAccCheckKubernetesPodForceNew(&conf1, &conf2, true),
				),
			},
		},
	})
}

func TestAccKubernetesPod_with_pod_security_context(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithSecurityContext(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.fs_group", "100"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.run_as_non_root", "true"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.run_as_user", "101"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.supplemental_groups.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.supplemental_groups.0", "101"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_pod_security_context_fs_group_change_policy(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t); skipIfUnsupportedSecurityContextRunAsGroup(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithSecurityContextFSChangePolicy(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.fs_group", "100"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.run_as_group", "100"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.run_as_non_root", "true"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.run_as_user", "101"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.fs_group_change_policy", "OnRootMismatch"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func testAccKubernetesPodConfigWithSecurityContextFSChangePolicy(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }
    name = "%[1]s"
  }
  spec {
    security_context {
      fs_group               = 100
      run_as_group           = 100
      run_as_non_root        = true
      run_as_user            = 101
      fs_group_change_policy = "OnRootMismatch"
    }
    container {
      image = "%[2]s"
      name  = "containername"
    }
  }
}
`, podName, imageName)
}

func TestAccKubernetesPod_with_pod_security_context_run_as_group(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t); skipIfUnsupportedSecurityContextRunAsGroup(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithSecurityContextRunAsGroup(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.fs_group", "100"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.run_as_group", "100"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.run_as_non_root", "true"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.run_as_user", "101"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.supplemental_groups.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.supplemental_groups.0", "101"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_pod_security_context_seccomp_profile(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithSecurityContextSeccompProfile(podName, imageName, "Unconfined"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.seccomp_profile.0.type", "Unconfined"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.security_context.0.seccomp_profile.0.type", "Unconfined"),
				),
			},
			{
				Config: testAccKubernetesPodConfigWithSecurityContextSeccompProfile(podName, imageName, "RuntimeDefault"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.seccomp_profile.0.type", "RuntimeDefault"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.security_context.0.seccomp_profile.0.type", "RuntimeDefault"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_pod_security_context_seccomp_localhost_profile(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t); skipIfNotRunningInKind(t); skipIfClusterVersionLessThan(t, "1.19.0") },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithSecurityContextSeccompProfileLocalhost(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.seccomp_profile.0.type", "Localhost"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.security_context.0.seccomp_profile.0.localhost_profile", "profiles/audit.json"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.security_context.0.seccomp_profile.0.type", "Localhost"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.security_context.0.seccomp_profile.0.localhost_profile", "profiles/audit.json"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_container_liveness_probe_using_exec(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := "gcr.io/google_containers/busybox"
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithLivenessProbeUsingExec(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.args.#", "3"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.exec.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.exec.0.command.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.exec.0.command.0", "cat"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.exec.0.command.1", "/tmp/healthy"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.failure_threshold", "3"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.initial_delay_seconds", "5"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_container_liveness_probe_using_http_get(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := "registry.k8s.io/e2e-test-images/agnhost:2.40"
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithLivenessProbeUsingHTTPGet(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.args.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.http_get.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.http_get.0.path", "/healthz"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.http_get.0.port", "8080"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.http_get.0.http_header.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.http_get.0.http_header.0.name", "X-Custom-Header"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.http_get.0.http_header.0.value", "Awesome"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.initial_delay_seconds", "3"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_container_liveness_probe_using_tcp(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := "registry.k8s.io/e2e-test-images/agnhost:2.40"
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithLivenessProbeUsingTCP(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.args.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.tcp_socket.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.tcp_socket.0.port", "8080"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_container_liveness_probe_using_grpc(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := "registry.k8s.io/e2e-test-images/agnhost:2.40"
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipIfClusterVersionLessThan(t, "1.24.0")
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithLivenessProbeUsingGRPC(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.args.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.grpc.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.grpc.0.port", "8888"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.liveness_probe.0.grpc.0.service", "EchoService"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_container_lifecycle(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := "registry.k8s.io/e2e-test-images/agnhost:2.40"
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithLifeCycle(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.lifecycle.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.lifecycle.0.post_start.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.lifecycle.0.post_start.0.exec.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.lifecycle.0.post_start.0.exec.0.command.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.lifecycle.0.post_start.0.exec.0.command.0", "ls"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.lifecycle.0.post_start.0.exec.0.command.1", "-al"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.lifecycle.0.pre_stop.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.lifecycle.0.pre_stop.0.exec.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.lifecycle.0.pre_stop.0.exec.0.command.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.lifecycle.0.pre_stop.0.exec.0.command.0", "date"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_container_security_context(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithContainerSecurityContext(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.security_context.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.security_context.0.privileged", "true"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.security_context.0.run_as_user", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.security_context.0.se_linux_options.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.security_context.0.se_linux_options.0.level", "s0:c123,c456"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.security_context.0.capabilities.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.security_context.0.capabilities.0.add.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.security_context.0.capabilities.0.add.0", "NET_ADMIN"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.security_context.0.capabilities.0.add.1", "SYS_TIME"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_volume_mount(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	secretName := acctest.RandomWithPrefix("tf-acc-test")

	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithVolumeMounts(secretName, podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.mount_path", "/tmp/my_path"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.name", "db"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.read_only", "false"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.sub_path", ""),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.mount_propagation", "HostToContainer"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_cfg_map_volume_mount(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	cfgMap := acctest.RandomWithPrefix("tf-acc-test")
	imageName := busyboxImageVersion
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithConfigMapVolume(cfgMap, podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.mount_path", "/tmp/my_path"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.name", "cfg"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.read_only", "false"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.sub_path", ""),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.mount_propagation", "None"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.1.mount_path", "/tmp/my_raw_path"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.1.name", "cfg-binary"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.1.read_only", "false"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.1.sub_path", ""),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.name", "cfg"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.config_map.0.name", cfgMap),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.config_map.0.default_mode", "0777")),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_csi_volume_hostpath(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	secretName := acctest.RandomWithPrefix("tf-acc-test")
	volumeName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := "busybox:1.32"
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if err := testAccCheckCSIDriverExists("hostpath.csi.k8s.io"); err != nil {
				t.Skip(err.Error())
			}
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodCSIVolume(imageName, podName, secretName, volumeName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.mount_path", "/volume"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.name", volumeName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.read_only", "true"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.name", volumeName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.csi.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.csi.0.driver", "hostpath.csi.k8s.io"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.csi.0.read_only", "true"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.csi.0.node_publish_secret_ref.0.name", secretName),
				),
			},
		},
	})
}

func TestAccKubernetesPod_with_projected_volume(t *testing.T) {
	var conf api.Pod

	cfgMapName := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	cfgMap2Name := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	secretName := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	podName := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	imageName := busyboxImageVersion
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodProjectedVolume(cfgMapName, cfgMap2Name, secretName, podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.name", "projected-vol"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.default_mode", "0777"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.#", "4"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.0.config_map.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.0.config_map.0.name", cfgMapName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.1.config_map.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.1.config_map.0.name", cfgMap2Name),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.2.secret.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.2.secret.0.name", secretName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.3.downward_api.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.3.downward_api.0.items.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.3.downward_api.0.items.0.path", "labels"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.3.downward_api.0.items.0.field_ref.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.3.downward_api.0.items.0.field_ref.0.field_path", "metadata.labels"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.3.downward_api.0.items.1.path", "cpu_limit"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.3.downward_api.0.items.1.resource_field_ref.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.3.downward_api.0.items.1.resource_field_ref.0.container_name", "containername"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.3.downward_api.0.items.1.resource_field_ref.0.resource", "limits.cpu"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.projected.0.sources.3.downward_api.0.items.1.resource_field_ref.0.divisor", "1"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_resource_requirements(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithResourceRequirements(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.resources.0.requests.memory", "50Mi"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.resources.0.requests.cpu", "250m"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.resources.0.requests.ephemeral-storage", "128Mi"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.resources.0.limits.memory", "512Mi"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.resources.0.limits.cpu", "500m"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.resources.0.limits.ephemeral-storage", "512Mi"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
			{
				Config: testAccKubernetesPodConfigWithEmptyResourceRequirements(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.resources.0.requests.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.resources.0.limits.#", "0"),
				),
			},
			{
				Config: testAccKubernetesPodConfigWithResourceRequirementsLimitsOnly(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.resources.0.requests.memory", "512Mi"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.resources.0.requests.cpu", "500m"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.resources.0.limits.memory", "512Mi"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.resources.0.limits.cpu", "500m"),
				),
			},
			{
				Config: testAccKubernetesPodConfigWithResourceRequirementsRequestsOnly(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.resources.0.requests.memory", "512Mi"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.resources.0.requests.cpu", "500m"),
				),
			},
		},
	})
}

func TestAccKubernetesPod_with_empty_dir_volume(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithEmptyDirVolumes(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.mount_path", "/cache"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.name", "cache-volume"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.empty_dir.0.medium", "Memory"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_empty_dir_volume_with_sizeLimit(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithEmptyDirVolumesSizeLimit(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.mount_path", "/cache"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.volume_mount.0.name", "cache-volume"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.empty_dir.0.medium", "Memory"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.empty_dir.0.size_limit", "512Mi"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_secret_vol_items(t *testing.T) {
	var conf api.Pod

	secretName := acctest.RandomWithPrefix("tf-acc-test")
	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithSecretItemsVolume(secretName, podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.secret.0.items.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.secret.0.items.0.key", "one"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.volume.0.secret.0.items.0.path", "path/to/one"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_gke_with_nodeSelector(t *testing.T) {
	var conf api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := alpineImage
	region := os.Getenv("GOOGLE_REGION")
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t); skipIfNotRunningInGke(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigNodeSelector(podName, imageName, region),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.image", imageName),
					resource.TestCheckResourceAttr(resourceName, "spec.0.node_selector.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.node_selector.failure-domain.beta.kubernetes.io/region", region),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_config_with_automount_service_account_token(t *testing.T) {
	var confPod api.Pod
	var confSA api.ServiceAccount

	podName := acctest.RandomWithPrefix("tf-acc-test")
	saName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWithAutomountServiceAccountToken(saName, podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesServiceAccountExists(fmt.Sprintf("kubernetes_service_account.%s", saName), &confSA),
					testAccCheckKubernetesPodExists(resourceName, &confPod),
					resource.TestCheckResourceAttr(resourceName, "spec.0.automount_service_account_token", "true"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_config_container_working_dir(t *testing.T) {
	var confPod api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigWorkingDir(podName, imageName, "/www"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &confPod),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.generation", "0"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.working_dir", "/www"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
			{
				Config: testAccKubernetesPodConfigWorkingDir(podName, imageName, "/srv"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &confPod),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.generation", "0"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.working_dir", "/srv"),
				),
			},
		},
	})
}

func TestAccKubernetesPod_config_container_startup_probe(t *testing.T) {
	var confPod api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipIfClusterVersionLessThan(t, "1.17.0")
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodContainerStartupProbe(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &confPod),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.generation", "0"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.startup_probe.0.http_get.0.path", "/index.html"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.startup_probe.0.http_get.0.port", "80"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.startup_probe.0.initial_delay_seconds", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.startup_probe.0.timeout_seconds", "2"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_termination_message_policy_default(t *testing.T) {
	var confPod api.Pod

	podName := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesTerminationMessagePolicyDefault(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &confPod),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.termination_message_policy", "File"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_termination_message_policy_override_as_file(t *testing.T) {
	var confPod api.Pod

	podName := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesTerminationMessagePolicyWithFile(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &confPod),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.termination_message_policy", "File"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_termination_message_policy_override_as_fallback_to_logs_on_err(t *testing.T) {
	var confPod api.Pod

	podName := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesTerminationMessagePolicyWithFallBackToLogsOnErr(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &confPod),
					resource.TestCheckResourceAttr(resourceName, "spec.0.container.0.termination_message_policy", "FallbackToLogsOnError"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_enableServiceLinks(t *testing.T) {
	var conf1 api.Pod

	rName := acctest.RandomWithPrefix("tf-acc-test")
	imageName := alpineImage
	resourceName := fmt.Sprintf("kubernetes_pod.%s", rName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigEnableServiceLinks(rName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf1),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.annotations.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.labels.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.labels.app", "pod_label"),
					resource.TestCheckResourceAttr(resourceName, "metadata.0.name", rName),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.generation"),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.resource_version"),
					resource.TestCheckResourceAttrSet(resourceName, "metadata.0.uid"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.enable_service_links", "false"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_bug961EmptyBlocks(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				ExpectError: regexp.MustCompile("Missing required argument"),
				Config:      testAccKubernetesPodConfigEmptyBlocks(name, busyboxImageVersion),
			},
		},
	})
}

func TestAccKubernetesPod_readinessGate(t *testing.T) {
	var conf1 api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	secretName := acctest.RandomWithPrefix("tf-acc-test")
	configMapName := acctest.RandomWithPrefix("tf-acc-test")
	imageName1 := nginxImageVersion1
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigBasic(secretName, configMapName, podName, imageName1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf1),
				),
			},
			{
				Config: testAccKubernetesPodConfigReadinessGate(secretName, configMapName, podName, imageName1),
				PreConfig: func() {
					conn, err := testAccProvider.Meta().(KubeClientsets).MainClientset()
					if err != nil {
						t.Fatal(err)
					}
					ctx := context.TODO()

					conditions := conf1.Status.Conditions
					testCondition := api.PodCondition{
						Type:   api.PodConditionType("haha"),
						Status: api.ConditionTrue,
					}
					updatedConditions := append(conditions, testCondition)
					conf1.Status.Conditions = updatedConditions
					_, err = conn.CoreV1().Pods("default").UpdateStatus(ctx, &conf1, metav1.UpdateOptions{})
					if err != nil {
						t.Fatal(err)
					}
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf1),
					resource.TestCheckResourceAttr(resourceName, "spec.0.readiness_gate.0.condition_type", "haha"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_topologySpreadConstraint(t *testing.T) {
	var conf1 api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := fmt.Sprintf("kubernetes_pod.%s", podName)
	imageName := "nginx:1.7.9"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipIfClusterVersionGreaterThanOrEqual(t, "1.17.0")
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodTopologySpreadConstraintConfig(podName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf1),
					resource.TestCheckResourceAttr(resourceName, "spec.0.topology_spread_constraint.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.topology_spread_constraint.0.max_skew", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.topology_spread_constraint.0.topology_key", "failure-domain.beta.kubernetes.io/zone"),
					resource.TestCheckResourceAttr(resourceName, "spec.0.topology_spread_constraint.0.when_unsatisfiable", "ScheduleAnyway"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_runtimeClassName(t *testing.T) {
	var conf1 api.Pod

	podName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := fmt.Sprintf("kubernetes_pod_v1.%s", podName)
	runtimeHandler := fmt.Sprintf("runc-%s", podName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipIfRunningInEks(t)
			createRuncRuntimeClass(runtimeHandler)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			err := deleteRuntimeClass(runtimeHandler)
			if err != nil {
				return err
			}
			return testAccCheckKubernetesPodDestroy(s)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodConfigRuntimeClassName(podName, busyboxImageVersion, runtimeHandler),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists(resourceName, &conf1),
					resource.TestCheckResourceAttr(resourceName, "spec.0.runtime_class_name", runtimeHandler),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func TestAccKubernetesPod_with_ephemeral_storage(t *testing.T) {
	var pod api.Pod
	var pvc api.PersistentVolumeClaim

	testName := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	imageName := nginxImageVersion

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKubernetesPodDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesPodEphemeralStorage(testName, imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesPodExists("kubernetes_pod_v1.test", &pod),
					testAccCheckKubernetesPersistentVolumeClaimCreated("default", testName+"-ephemeral", &pvc),
					resource.TestCheckResourceAttr("kubernetes_pod_v1.test", "spec.0.volume.0.name", "ephemeral"),
					resource.TestCheckResourceAttr("kubernetes_pod_v1.test", "spec.0.volume.0.ephemeral.0.spec.0.storage_class_name", testName),
				),
			},
			// Do a second test with only the storage class and check that the PVC has been deleted by the ephemeral volume
			{
				Config: testAccKubernetesPodEphemeralStorageWithoutPod(testName),
				Check:  testAccCheckKubernetesPersistentVolumeClaimIsDestroyed(&pvc),
			},
		},
	})
}

func createRuncRuntimeClass(rn string) error {
	conn, err := testAccProvider.Meta().(KubeClientsets).MainClientset()
	if err != nil {
		return err
	}
	_, err = conn.NodeV1().RuntimeClasses().Create(context.Background(), &nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: rn,
		},
		Handler: "runc",
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func deleteRuntimeClass(rn string) error {
	conn, err := testAccProvider.Meta().(KubeClientsets).MainClientset()
	if err != nil {
		return err
	}
	return conn.NodeV1().RuntimeClasses().Delete(context.Background(), rn, metav1.DeleteOptions{})
}

func testAccCheckCSIDriverExists(csiDriverName string) error {
	conn, err := testAccProvider.Meta().(KubeClientsets).MainClientset()
	if err != nil {
		return err
	}
	ctx := context.Background()
	_, err = conn.StorageV1().CSIDrivers().Get(ctx, csiDriverName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("could not find CSIDriver %q", csiDriverName)
	}
	return nil
}

func testAccCheckKubernetesPodDestroy(s *terraform.State) error {
	conn, err := testAccProvider.Meta().(KubeClientsets).MainClientset()

	if err != nil {
		return err
	}
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "kubernetes_pod" {
			continue
		}

		namespace, name, err := idParts(rs.Primary.ID)
		if err != nil {
			return err
		}

		resp, err := conn.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			if resp.Namespace == namespace && resp.Name == name {
				return fmt.Errorf("Pod still exists: %s: %#v", rs.Primary.ID, resp.Status.Phase)
			}
		}
	}

	return nil
}

func testAccCheckKubernetesPodExists(n string, obj *api.Pod) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn, err := testAccProvider.Meta().(KubeClientsets).MainClientset()
		if err != nil {
			return err
		}
		ctx := context.TODO()

		namespace, name, err := idParts(rs.Primary.ID)
		if err != nil {
			return err
		}

		out, err := conn.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		*obj = *out
		return nil
	}
}

func testAccCheckKubernetesPodForceNew(old, new *api.Pod, wantNew bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if wantNew {
			if old.ObjectMeta.UID == new.ObjectMeta.UID {
				return fmt.Errorf("Expecting new resource for pod %s", old.ObjectMeta.UID)
			}
		} else {
			if old.ObjectMeta.UID != new.ObjectMeta.UID {
				return fmt.Errorf("Expecting pod UIDs to be the same: expected %s got %s", old.ObjectMeta.UID, new.ObjectMeta.UID)
			}
		}
		return nil
	}
}

func testAccKubernetesPodConfigBasic(secretName, configMapName, podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_secret" "%[1]s" {
  metadata {
    name = "%[1]s"
  }

  data = {
    one = "first"
  }
}

resource "kubernetes_secret" "%[1]s_from" {
  metadata {
    name = "%[1]s-from"
  }

  data = {
    one    = "first_from"
    second = "second_from"
  }
}

resource "kubernetes_config_map" "%[2]s" {
  metadata {
    name = "%[2]s"
  }

  data = {
    one = "ONE"
  }
}

resource "kubernetes_config_map" "%[2]s_from" {
  metadata {
    name = "%[2]s-from"
  }

  data = {
    one = "ONE_FROM"
    two = "TWO_FROM"
  }
}

resource "kubernetes_pod" "%[3]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[3]s"
  }

  spec {
    automount_service_account_token = false
    container {
      image = "%[4]s"
      name  = "containername"

      env {
        name = "EXPORTED_VARIABLE_FROM_SECRET"

        value_from {
          secret_key_ref {
            name     = "${kubernetes_secret.%[1]s.metadata.0.name}"
            key      = "one"
            optional = true
          }
        }
      }
      env {
        name = "EXPORTED_VARIABLE_FROM_CONFIG_MAP"
        value_from {
          config_map_key_ref {
            name     = "${kubernetes_config_map.%[2]s.metadata.0.name}"
            key      = "one"
            optional = true
          }
        }
      }

      env_from {
        config_map_ref {
          name     = "${kubernetes_config_map.%[2]s_from.metadata.0.name}"
          optional = true
        }
        prefix = "FROM_CM_"
      }
      env_from {
        secret_ref {
          name     = "${kubernetes_secret.%[1]s_from.metadata.0.name}"
          optional = false
        }
        prefix = "FROM_S_"
      }
    }

    volume {
      name = "db"

      secret {
        secret_name = "${kubernetes_secret.%[1]s.metadata.0.name}"
      }
    }
  }
}
`, secretName, configMapName, podName, imageName)
}

func testAccKubernetesPodConfigScheduler(podName, schedulerName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod_v1" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }
    name = "%[1]s"
  }

  spec {
    automount_service_account_token = false
    scheduler_name                  = %[2]q
    container {
      image = %[3]q
      name  = "containername"
    }
  }
  timeouts {
    create = "1m"
  }
}
`, podName, schedulerName, imageName)
}

func testAccKubernetesPodConfigWithInitContainer(podName, image string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    name = "%[1]s"
    labels = {
      "app.kubernetes.io/name" = "acctest"
    }
  }

  spec {
    automount_service_account_token = false
    container {
      name    = "container"
      image   = "%[2]s"
      command = ["sh", "-c", "echo The app is running! && sleep 3600"]

      resources {
        requests = {
          memory = "64Mi"
          cpu    = "50m"
        }
      }
    }

    init_container {
      name    = "initcontainer"
      image   = "%[2]s"
      command = ["sh", "-c", "until nslookup %[1]s-init-service.default.svc.cluster.local; do echo waiting for init-service; sleep 2; done"]

      resources {
        requests = {
          memory = "64Mi"
          cpu    = "50m"
        }
      }
    }
  }
}

resource "kubernetes_service" "%[1]s" {
  metadata {
    name = "%[1]s-init-service"
  }

  spec {
    selector = {
      "app.kubernetes.io/name" = "acctest"
    }
    port {
      port        = 8080
      target_port = 80
    }
  }
}
`, podName, image)
}

func testAccKubernetesPodConfigWithSecurityContext(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    security_context {
      fs_group            = 100
      run_as_non_root     = true
      run_as_user         = 101
      supplemental_groups = [101]
    }

    container {
      image = "%[2]s"
      name  = "containername"
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigWithSecurityContextRunAsGroup(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    security_context {
      fs_group            = 100
      run_as_group        = 100
      run_as_non_root     = true
      run_as_user         = 101
      supplemental_groups = [101]
    }

    container {
      image = "%[2]s"
      name  = "containername"
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigWithSecurityContextSeccompProfile(podName, imageName, seccompProfileType string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    automount_service_account_token = false
    security_context {
      seccomp_profile {
        type = "%[2]s"
      }
    }

    container {
      image = "%[3]s"
      name  = "containername"
      security_context {
        seccomp_profile {
          type = "%[2]s"
        }
      }
    }
  }
}
`, podName, seccompProfileType, imageName)
}

func testAccKubernetesPodConfigWithSecurityContextSeccompProfileLocalhost(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    automount_service_account_token = false
    security_context {
      seccomp_profile {
        type              = "Localhost"
        localhost_profile = "profiles/audit.json"
      }
    }

    container {
      image = "%[2]s"
      name  = "containername"
      security_context {
        seccomp_profile {
          type              = "Localhost"
          localhost_profile = "profiles/audit.json"
        }
      }
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigWithLivenessProbeUsingExec(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      name  = "containername"
      args  = ["/bin/sh", "-c", "touch /tmp/healthy; sleep 300; rm -rf /tmp/healthy; sleep 600"]

      liveness_probe {
        exec {
          command = ["cat", "/tmp/healthy"]
        }

        initial_delay_seconds = 5
        period_seconds        = 5
      }
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigWithLivenessProbeUsingHTTPGet(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      name  = "containername"
      args  = ["liveness"]

      liveness_probe {
        http_get {
          path = "/healthz"
          port = 8080

          http_header {
            name  = "X-Custom-Header"
            value = "Awesome"
          }
        }

        initial_delay_seconds = 3
        period_seconds        = 3
      }
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigWithLivenessProbeUsingTCP(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      name  = "containername"
      args  = ["liveness"]

      liveness_probe {
        tcp_socket {
          port = 8080
        }

        initial_delay_seconds = 3
        period_seconds        = 3
      }
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigWithLivenessProbeUsingGRPC(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      name  = "containername"
      args  = ["liveness"]

      liveness_probe {
        grpc {
          port    = 8888
          service = "EchoService"
        }

        initial_delay_seconds = 30
        period_seconds        = 30
      }
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigWithLifeCycle(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      name  = "containername"
      args  = ["liveness"]

      lifecycle {
        post_start {
          exec {
            command = ["ls", "-al"]
          }
        }

        pre_stop {
          exec {
            command = ["date"]
          }
        }
      }
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigWithContainerSecurityContext(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      name  = "containername"

      security_context {
        privileged  = true
        run_as_user = 1

        se_linux_options {
          level = "s0:c123,c456"
        }

        capabilities {
          add = ["NET_ADMIN", "SYS_TIME"]
        }
      }
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigWithVolumeMounts(secretName, podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_secret" "%[1]s" {
  metadata {
    name = "%[1]s"
  }

  data = {
    one = "first"
  }
}

resource "kubernetes_pod" "%[2]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[2]s"
  }

  spec {
    automount_service_account_token = false

    container {
      image = "%[3]s"
      name  = "containername"

      volume_mount {
        mount_path        = "/tmp/my_path"
        name              = "db"
        mount_propagation = "HostToContainer"
      }
    }

    volume {
      name = "db"

      secret {
        secret_name = "${kubernetes_secret.%[1]s.metadata.0.name}"
      }
    }
  }
}
`, secretName, podName, imageName)
}

func testAccKubernetesPodConfigWithSecretItemsVolume(secretName, podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_secret" "%[1]s" {
  metadata {
    name = "%[1]s"
  }

  data = {
    one = "first"
  }
}

resource "kubernetes_pod" "%[2]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[2]s"
  }

  spec {
    automount_service_account_token = false

    container {
      image = "%[3]s"
      name  = "containername"

      volume_mount {
        mount_path = "/tmp/my_path"
        name       = "db"
      }
    }

    volume {
      name = "db"

      secret {
        secret_name = "${kubernetes_secret.%[1]s.metadata.0.name}"

        items {
          key  = "one"
          path = "path/to/one"
        }
      }
    }
  }
}
`, secretName, podName, imageName)
}

func testAccKubernetesPodConfigWithConfigMapVolume(secretName, podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_config_map" "%[1]s" {
  metadata {
    name = "%[1]s"
  }

  binary_data = {
    raw = "${base64encode("Raw data should come back as is in the pod")}"
  }

  data = {
    one = "first"
  }
}

resource "kubernetes_pod" "%[2]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[2]s"
  }

  spec {
    restart_policy                  = "Never"
    automount_service_account_token = false

    container {
      image = "%[3]s"
      name  = "containername"

      args = ["/bin/sh", "-xc", "ls -l /tmp/my_raw_path ; cat /tmp/my_raw_path/raw.txt ; sleep 10"]

      lifecycle {
        post_start {
          exec {
            command = ["/bin/sh", "-xc", "grep 'Raw data should come back as is in the pod' /tmp/my_raw_path/raw.txt"]
          }
        }
      }

      volume_mount {
        mount_path = "/tmp/my_path"
        name       = "cfg"
      }

      volume_mount {
        mount_path = "/tmp/my_raw_path"
        name       = "cfg-binary"
      }
    }

    volume {
      name = "cfg"

      config_map {
        name         = "${kubernetes_config_map.%[1]s.metadata.0.name}"
        default_mode = "0777"
      }
    }

    volume {
      name = "cfg-item"

      config_map {
        name = "${kubernetes_config_map.%[1]s.metadata.0.name}"

        items {
          key  = "one"
          path = "one.txt"
        }
      }
    }

    volume {
      name = "cfg-item-with-mode"

      config_map {
        name = "${kubernetes_config_map.%[1]s.metadata.0.name}"

        items {
          key  = "one"
          path = "one-with-mode.txt"
          mode = "0444"
        }
      }
    }

    volume {
      name = "cfg-binary"

      config_map {
        name = "${kubernetes_config_map.%[1]s.metadata.0.name}"

        items {
          key  = "raw"
          path = "raw.txt"
        }
      }
    }
  }
}
`, secretName, podName, imageName)
}

func testAccKubernetesPodCSIVolume(imageName, podName, secretName, volumeName string) string {
	return fmt.Sprintf(`resource "kubernetes_secret" "%[3]s" {
  metadata {
    name = "%[3]s"
  }

  data = {
    secret = "%[3]s"
  }
}

resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      "label" = "web"
    }
    name = "%[1]s"
  }
  spec {
    container {
      image   = "%[2]s"
      name    = "%[1]s"
      command = ["sleep", "36000"]
      volume_mount {
        name       = "%[4]s"
        mount_path = "/volume"
        read_only  = true
      }
    }
    restart_policy = "Never"
    volume {
      name = "%[4]s"
      csi {
        driver    = "hostpath.csi.k8s.io"
        read_only = true
        volume_attributes = {
          "secretProviderClass" = "secret-provider"
        }
        node_publish_secret_ref {
          name = "%[3]s"
        }
      }
    }
  }
}`, podName, imageName, secretName, volumeName)
}

func testAccKubernetesPodProjectedVolume(cfgMapName, cfgMap2Name, secretName, podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_config_map" "%[1]s" {
  metadata {
    name = "%[1]s"
  }

  binary_data = {
    raw = "${base64encode("Raw data should come back as is in the pod")}"
  }

  data = {
    one = "first"
  }
}

resource "kubernetes_config_map" "%[2]s" {
  metadata {
    name = "%[2]s"
  }

  binary_data = {
    raw = "${base64encode("Raw data should come back as is in the pod")}"
  }

  data = {
    one = "first"
  }
}

resource "kubernetes_secret" "%[3]s" {
  metadata {
    name = "%[3]s"
  }

  data = {
    one = "first"
  }
}

resource "kubernetes_pod" "%[4]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[4]s"
  }

  spec {
    restart_policy                  = "Never"
    automount_service_account_token = false

    container {
      image = "%[5]s"
      name  = "containername"

      command = ["sleep", "3600"]

      lifecycle {
        post_start {
          exec {
            command = ["/bin/sh", "-xc", "grep 'Raw data should come back as is in the pod' /tmp/my-projected-volume/raw.txt"]
          }
        }
      }

      volume_mount {
        mount_path = "/tmp/my-projected-volume"
        name       = "projected-vol"
      }
    }

    volume {
      name = "projected-vol"
      projected {
        default_mode = "0777"
        sources {
          config_map {
            name = "${kubernetes_config_map.%[1]s.metadata.0.name}"
            items {
              key  = "raw"
              path = "raw.txt"
            }
          }
        }
        sources {
          config_map {
            name = "${kubernetes_config_map.%[2]s.metadata.0.name}"
            items {
              key  = "raw"
              path = "raw-again.txt"
            }
          }
        }
        sources {
          secret {
            name = "${kubernetes_secret.%[3]s.metadata.0.name}"
            items {
              key  = "one"
              path = "secret.txt"
            }
          }
        }
        sources {
          downward_api {
            items {
              path = "labels"
              field_ref {
                field_path = "metadata.labels"
              }
            }
            items {
              path = "cpu_limit"
              resource_field_ref {
                container_name = "containername"
                resource       = "limits.cpu"
              }
            }
          }
        }
      }
    }
  }
}
`, cfgMapName, cfgMap2Name, secretName, podName, imageName)
}

func testAccKubernetesPodConfigWithResourceRequirements(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      name  = "containername"

      resources {
        limits = {
          cpu                 = "0.5"
          memory              = "512Mi"
          "ephemeral-storage" = "512Mi"
        }

        requests = {
          cpu                 = "250m"
          memory              = "50Mi"
          "ephemeral-storage" = "128Mi"
        }
      }
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigWithEmptyResourceRequirements(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      name  = "containername"

      resources {
        limits   = {}
        requests = {}
      }
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigWithResourceRequirementsLimitsOnly(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      name  = "containername"

      resources {
        limits = {
          cpu    = "500m"
          memory = "512Mi"
        }
      }
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigWithResourceRequirementsRequestsOnly(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      name  = "containername"

      resources {
        requests = {
          cpu    = "500m"
          memory = "512Mi"
        }
      }
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigWithEmptyDirVolumes(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    automount_service_account_token = false

    container {
      image = "%[2]s"
      name  = "containername"

      volume_mount {
        mount_path = "/cache"
        name       = "cache-volume"
      }
    }

    volume {
      name = "cache-volume"

      empty_dir {
        medium = "Memory"
      }
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigWithEmptyDirVolumesSizeLimit(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    automount_service_account_token = false

    container {
      image = "%[2]s"
      name  = "containername"

      volume_mount {
        mount_path = "/cache"
        name       = "cache-volume"
      }
    }

    volume {
      name = "cache-volume"

      empty_dir {
        medium     = "Memory"
        size_limit = "512Mi"
      }
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigNodeSelector(podName, imageName, region string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      name  = "containername"
    }

    node_selector = {
      "failure-domain.beta.kubernetes.io/region" = "%[3]s"
    }
  }
}
`, podName, imageName, region)
}

func testAccKubernetesPodConfigArgsUpdate(podName, imageName, args string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      args  = %[3]s
      name  = "containername"
    }
  }
}
`, podName, imageName, args)
}

func testAccKubernetesPodConfigEnvUpdate(podName, imageName, val string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    name = "%[1]s"
  }

  spec {
    container {
      image = "%[1]s"
      name  = "containername"

      env {
        name  = "foo"
        value = "%[3]s"
      }
    }
  }
}
`, podName, imageName, val)
}

func testAccKubernetesPodConfigWithAutomountServiceAccountToken(saName string, podName string, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_service_account" "%[1]s" {
  metadata {
    name = "%[1]s"
  }
}

resource "kubernetes_pod" "%[2]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[2]s"
  }

  spec {
    service_account_name            = kubernetes_service_account.%[1]s.metadata.0.name
    automount_service_account_token = true

    container {
      image = "%[3]s"
      name  = "containername"

      lifecycle {
        post_start {
          exec {
            command = ["/bin/sh", "-xc", "mount | grep /run/secrets/kubernetes.io/serviceaccount"]
          }
        }
      }
    }
  }
}
`, saName, podName, imageName)
}

func testAccKubernetesPodConfigWorkingDir(podName, imageName, val string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    name = "%[1]s"
  }

  spec {
    container {
      image       = "%[2]s"
      name        = "containername"
      working_dir = "%[3]s"
    }
  }
}
`, podName, imageName, val)
}

func testAccKubernetesPodContainerStartupProbe(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      name  = "containername"

      startup_probe {
        http_get {
          path = "/index.html"
          port = 80
        }

        initial_delay_seconds = 1
        timeout_seconds       = 2
      }
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesTerminationMessagePolicyDefault(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      name  = "containername"
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesTerminationMessagePolicyWithOverride(podName, imageName, terminationMessagePolicy string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    name = "%[1]s"
  }

  spec {
    container {
      image                      = "%[2]s"
      name                       = "containername"
      termination_message_policy = "%[3]s"
    }
  }
}
`, podName, imageName, terminationMessagePolicy)
}

func testAccKubernetesTerminationMessagePolicyWithFile(podName, imageName string) string {
	return testAccKubernetesTerminationMessagePolicyWithOverride(podName, imageName, "File")
}

func testAccKubernetesTerminationMessagePolicyWithFallBackToLogsOnErr(podName, imageName string) string {
	return testAccKubernetesTerminationMessagePolicyWithOverride(podName, imageName, "FallbackToLogsOnError")
}

func testAccKubernetesPodConfigEnableServiceLinks(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      name  = "containername"
    }
    enable_service_links = false
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigReadinessGate(secretName, configMapName, podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_secret" "%[1]s" {
  metadata {
    name = "%[1]s"
  }

  data = {
    one = "first"
  }
}

resource "kubernetes_secret" "%[1]s_from" {
  metadata {
    name = "%[1]s-from"
  }

  data = {
    one    = "first_from"
    second = "second_from"
  }
}

resource "kubernetes_config_map" "%[2]s" {
  metadata {
    name = "%[2]s"
  }

  data = {
    one = "ONE"
  }
}

resource "kubernetes_config_map" "%[2]s_from" {
  metadata {
    name = "%[2]s-from"
  }

  data = {
    one = "ONE_FROM"
    two = "TWO_FROM"
  }
}

resource "kubernetes_pod" "%[3]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[3]s"
  }

  spec {
    automount_service_account_token = false

    readiness_gate {
      condition_type = "haha"
    }
    container {
      image = "%[4]s"
      name  = "containername"

      env {
        name = "EXPORTED_VARIABLE_FROM_SECRET"

        value_from {
          secret_key_ref {
            name     = "${kubernetes_secret.%[1]s.metadata.0.name}"
            key      = "one"
            optional = true
          }
        }
      }
      env {
        name = "EXPORTED_VARIABLE_FROM_CONFIG_MAP"
        value_from {
          config_map_key_ref {
            name     = "${kubernetes_config_map.%[2]s.metadata.0.name}"
            key      = "one"
            optional = true
          }
        }
      }

      env_from {
        config_map_ref {
          name     = "${kubernetes_config_map.%[2]s_from.metadata.0.name}"
          optional = true
        }
        prefix = "FROM_CM_"
      }
      env_from {
        secret_ref {
          name     = "${kubernetes_secret.%[1]s_from.metadata.0.name}"
          optional = false
        }
        prefix = "FROM_S_"
      }
    }

    volume {
      name = "db"

      secret {
        secret_name = "${kubernetes_secret.%[1]s.metadata.0.name}"
      }
    }
  }
}
`, secretName, configMapName, podName, imageName)
}

func testAccKubernetesPodConfigMinimal(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    name = "%[1]s"
  }
  spec {
    container {
      image = "%[2]s"
      name  = "containername"
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigEmptyBlocks(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    labels = {
      app = "pod_label"
    }

    name = "%[1]s"
  }

  spec {
    container {
      image = "%[2]s"
      name  = "containername"

      env {}
      env_from {
        config_map_ref {}
      }
      env_from {
        secret_ref {}
      }
      env_from {}
    }
    volume {
      name = "empty"
      secret {}
    }
    volume {}
  }
}
`, podName, imageName)
}

func testAccKubernetesPodTopologySpreadConstraintConfig(podName, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_pod" "%[1]s" {
  metadata {
    name = "%[1]s"
  }
  spec {
    container {
      image = "%[2]s"
      name  = "containername"
    }
    topology_spread_constraint {
      max_skew           = 1
      topology_key       = "failure-domain.beta.kubernetes.io/zone"
      when_unsatisfiable = "ScheduleAnyway"
      label_selector {
        match_labels = {
          "app.kubernetes.io/instance" = "terraform-example"
        }
      }
    }
  }
}
`, podName, imageName)
}

func testAccKubernetesPodConfigRuntimeClassName(podName, imageName, runtimeHandler string) string {
	return fmt.Sprintf(`resource "kubernetes_pod_v1" "%[1]s" {
  metadata {
    name = "%[1]s"
  }
  spec {
    runtime_class_name = "%[2]s"
    container {
      image = "%[3]s"
      name  = "containername"
    }
  }
}
`, podName, runtimeHandler, imageName)
}

func testAccKubernetesCustomScheduler(name string) string {
	// Source: https://kubernetes.io/docs/tasks/extend-kubernetes/configure-multiple-schedulers/
	return fmt.Sprintf(`variable "namespace" {
  default = "kube-system"
}

variable "scheduler_name" {
  default = %q
}

variable "scheduler_cluster_version" {
  default = ""
}

resource "kubernetes_service_account_v1" "scheduler" {
  metadata {
    name      = var.scheduler_name
    namespace = var.namespace
  }
}

resource "kubernetes_cluster_role_binding_v1" "kube_scheduler" {
  metadata {
    name = "${var.scheduler_name}-as-kube-scheduler"
  }
  subject {
    kind      = "ServiceAccount"
    name      = var.scheduler_name
    namespace = var.namespace
  }
  role_ref {
    kind      = "ClusterRole"
    name      = "system:kube-scheduler"
    api_group = "rbac.authorization.k8s.io"
  }
}

resource "kubernetes_cluster_role_binding_v1" "volume_scheduler" {
  metadata {
    name = "${var.scheduler_name}-as-volume-scheduler"
  }
  subject {
    kind      = "ServiceAccount"
    name      = var.scheduler_name
    namespace = var.namespace
  }
  role_ref {
    kind      = "ClusterRole"
    name      = "system:volume-scheduler"
    api_group = "rbac.authorization.k8s.io"
  }
}

resource "kubernetes_role_binding_v1" "authentication_reader" {
  metadata {
    name      = "${var.scheduler_name}-extension-apiserver-authentication-reader"
    namespace = var.namespace
  }
  role_ref {
    kind      = "Role"
    name      = "extension-apiserver-authentication-reader"
    api_group = "rbac.authorization.k8s.io"
  }
  subject {
    kind      = "ServiceAccount"
    name      = var.scheduler_name
    namespace = var.namespace
  }
}

resource "kubernetes_config_map_v1" "scheduler_config" {
  metadata {
    name      = "${var.scheduler_name}-config"
    namespace = var.namespace
  }
  data = {
    "scheduler-config.yaml" = yamlencode(
      {
        "apiVersion" : "kubescheduler.config.k8s.io/v1beta2",
        "kind" : "KubeSchedulerConfiguration",
        profiles : [{
          "schedulerName" : var.scheduler_name
        }],
        "leaderElection" : { "leaderElect" : false }
      }
    )
  }
}

resource "kubernetes_pod_v1" "scheduler" {
  metadata {
    labels = {
      component = "scheduler"
      tier      = "control-plane"
    }
    name      = var.scheduler_name
    namespace = var.namespace
  }

  spec {
    service_account_name = kubernetes_service_account_v1.scheduler.metadata.0.name
    container {
      name = var.scheduler_name
      command = [
        "/usr/local/bin/kube-scheduler",
        "--config=/etc/kubernetes/scheduler/scheduler-config.yaml"
      ]
      image = "registry.k8s.io/kube-scheduler:${var.scheduler_cluster_version}"
      liveness_probe {
        http_get {
          path   = "/healthz"
          port   = 10259
          scheme = "HTTPS"
        }
        initial_delay_seconds = 15
      }
      resources {
        requests = {
          cpu = "0.1"
        }
      }
      security_context {
        privileged = false
      }
      volume_mount {
        name       = "config-volume"
        mount_path = "/etc/kubernetes/scheduler"
      }
    }
    volume {
      name = "config-volume"
      config_map {
        name = kubernetes_config_map_v1.scheduler_config.metadata.0.name
      }
    }
  }

  timeouts {
    create = "1m"
  }
}
`, name)
}

func testAccKubernetesPodEphemeralStorage(name, imageName string) string {
	return fmt.Sprintf(`resource "kubernetes_storage_class" "test" {
  metadata {
    name = %[1]q
  }
  storage_provisioner = "pd.csi.storage.gke.io"
  reclaim_policy      = "Delete"
  volume_binding_mode = "WaitForFirstConsumer"

  parameters = {
    type = "pd-standard"
  }
}

resource "kubernetes_pod_v1" "test" {
  metadata {
    name = %[1]q
    labels = {
      Test = "TfAcceptanceTest"
    }
  }
  spec {
    priority_class_name = "default"
    container {
      name  = "containername"
      image = %[2]q

      volume_mount {
        mount_path = "/ephemeral"
        name       = "ephemeral"
      }
    }

    volume {
      name = "ephemeral"

      ephemeral {
        metadata {
          labels = {
            Test = "TfAcceptanceTest"
          }
        }
        spec {
          access_modes       = ["ReadWriteOnce"]
          storage_class_name = %[1]q

          resources {
            requests = {
              storage = "5Gi"
            }
          }
        }
      }
    }
  }
}
`, name, imageName)
}

func testAccKubernetesPodEphemeralStorageWithoutPod(name string) string {
	return fmt.Sprintf(`resource "kubernetes_storage_class" "test" {
  metadata {
    name = %[1]q
  }
  storage_provisioner = "pd.csi.storage.gke.io"
  reclaim_policy      = "Delete"
  volume_binding_mode = "WaitForFirstConsumer"

  parameters = {
    type = "pd-standard"
  }
}
`, name)
}
