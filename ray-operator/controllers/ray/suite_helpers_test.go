package ray

import (
	"context"
	"log"
	"reflect"
	"time"

	"github.com/onsi/gomega"
	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	"github.com/ray-project/kuberay/ray-operator/controllers/ray/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getResourceFunc(ctx context.Context, key client.ObjectKey, obj client.Object) func() error {
	return func() error {
		return k8sClient.Get(ctx, key, obj)
	}
}

func listResourceFunc(ctx context.Context, workerPods *corev1.PodList, opt ...client.ListOption) func() (int, error) {
	return func() (int, error) {
		if err := k8sClient.List(ctx, workerPods, opt...); err != nil {
			return -1, err
		}

		count := 0
		for _, aPod := range workerPods.Items {
			if (reflect.DeepEqual(aPod.Status.Phase, corev1.PodRunning) || reflect.DeepEqual(aPod.Status.Phase, corev1.PodPending)) && aPod.DeletionTimestamp == nil {
				count++
			}
		}

		return count, nil
	}
}

func getClusterState(ctx context.Context, namespace string, clusterName string) func() rayv1.ClusterState {
	return func() rayv1.ClusterState {
		var cluster rayv1.RayCluster
		if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: clusterName}, &cluster); err != nil {
			log.Fatal(err)
		}
		return cluster.Status.State
	}
}

func isAllPodsRunning(ctx context.Context, podlist corev1.PodList, filterLabels client.MatchingLabels, namespace string) bool {
	err := k8sClient.List(ctx, &podlist, filterLabels, &client.ListOptions{Namespace: namespace})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed to list Pods")
	for _, pod := range podlist.Items {
		if pod.Status.Phase != corev1.PodRunning {
			return false
		}
	}
	return true
}

func cleanUpWorkersToDelete(ctx context.Context, rayCluster *rayv1.RayCluster, workerGroupIndex int) {
	// Updating WorkersToDelete is the responsibility of the Ray Autoscaler. In this function,
	// we simulate the behavior of the Ray Autoscaler after the scaling process has finished.
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		gomega.Eventually(
			getResourceFunc(ctx, client.ObjectKey{Name: rayCluster.Name, Namespace: "default"}, rayCluster),
			time.Second*9, time.Millisecond*500).Should(gomega.BeNil(), "raycluster = %v", rayCluster)
		rayCluster.Spec.WorkerGroupSpecs[workerGroupIndex].ScaleStrategy.WorkersToDelete = []string{}
		return k8sClient.Update(ctx, rayCluster)
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to clean up WorkersToDelete")
}

func getRayJobDeploymentStatus(ctx context.Context, rayJob *rayv1.RayJob) func() (rayv1.JobDeploymentStatus, error) {
	return func() (rayv1.JobDeploymentStatus, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: rayJob.Name, Namespace: "default"}, rayJob); err != nil {
			return "", err
		}
		return rayJob.Status.JobDeploymentStatus, nil
	}
}

func getRayClusterNameForRayJob(ctx context.Context, rayJob *rayv1.RayJob) func() (string, error) {
	return func() (string, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: rayJob.Name, Namespace: "default"}, rayJob); err != nil {
			return "", err
		}
		return rayJob.Status.RayClusterName, nil
	}
}

func getDashboardURLForRayJob(ctx context.Context, rayJob *rayv1.RayJob) func() (string, error) {
	return func() (string, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: rayJob.Name, Namespace: "default"}, rayJob); err != nil {
			return "", err
		}
		return rayJob.Status.DashboardURL, nil
	}
}

func prepareFakeRayDashboardClient() *utils.FakeRayDashboardClient {
	client := &utils.FakeRayDashboardClient{}

	healthyStatus := generateServeStatus(rayv1.DeploymentStatusEnum.HEALTHY, rayv1.ApplicationStatusEnum.RUNNING)
	client.SetMultiApplicationStatuses(map[string]*utils.ServeApplicationStatus{"app": &healthyStatus})

	return client
}

func generateServeStatus(deploymentStatus string, applicationStatus string) utils.ServeApplicationStatus {
	return utils.ServeApplicationStatus{
		Status: applicationStatus,
		Deployments: map[string]utils.ServeDeploymentStatus{
			"shallow": {
				Name:    "shallow",
				Status:  deploymentStatus,
				Message: "",
			},
			"deep": {
				Name:    "deep",
				Status:  deploymentStatus,
				Message: "",
			},
			"one": {
				Name:    "one",
				Status:  deploymentStatus,
				Message: "",
			},
		},
	}
}

func getRayClusterNameFunc(ctx context.Context, rayService *rayv1.RayService) func() (string, error) {
	return func() (string, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: rayService.Name, Namespace: "default"}, rayService); err != nil {
			return "", err
		}
		return rayService.Status.ActiveServiceStatus.RayClusterName, nil
	}
}

func getActiveRayClusterWorkerGroupSpecsFunc(ctx context.Context, rayService *rayv1.RayService) func() ([]rayv1.WorkerGroupSpec, error) {
	return func() ([]rayv1.WorkerGroupSpec, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: rayService.Name, Namespace: "default"}, rayService); err != nil {
			return nil, err
		}
		rayCluster := &rayv1.RayCluster{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: rayService.Status.ActiveServiceStatus.RayClusterName, Namespace: "default"}, rayCluster); err != nil {
			return nil, err
		}
		return rayCluster.Spec.WorkerGroupSpecs, nil
	}
}

func getPreparingRayClusterNameFunc(ctx context.Context, rayService *rayv1.RayService) func() (string, error) {
	return func() (string, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: rayService.Name, Namespace: "default"}, rayService); err != nil {
			return "", err
		}
		return rayService.Status.PendingServiceStatus.RayClusterName, nil
	}
}

func getPendingRayClusterWorkerGroupSpecsFunc(ctx context.Context, rayService *rayv1.RayService) func() ([]rayv1.WorkerGroupSpec, error) {
	return func() ([]rayv1.WorkerGroupSpec, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: rayService.Name, Namespace: "default"}, rayService); err != nil {
			return nil, err
		}
		rayCluster := &rayv1.RayCluster{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: rayService.Status.PendingServiceStatus.RayClusterName, Namespace: "default"}, rayCluster); err != nil {
			return nil, err
		}
		return rayCluster.Spec.WorkerGroupSpecs, nil
	}
}

func checkServiceHealth(ctx context.Context, rayService *rayv1.RayService) func() (bool, error) {
	return func() (bool, error) {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: rayService.Name, Namespace: rayService.Namespace}, rayService); err != nil {
			return false, err
		}

		for _, appStatus := range rayService.Status.ActiveServiceStatus.Applications {
			if appStatus.Status != rayv1.ApplicationStatusEnum.RUNNING {
				return false, nil
			}
			for _, deploymentStatus := range appStatus.Deployments {
				if deploymentStatus.Status != rayv1.DeploymentStatusEnum.HEALTHY {
					return false, nil
				}
			}
		}

		return true, nil
	}
}

// Update the status of the head Pod to Running.
// We need to manually update Pod statuses otherwise they'll always be Pending.
// envtest doesn't create a full K8s cluster. It's only the control plane.
// There's no container runtime or any other K8s controllers.
// So Pods are created, but no controller updates them from Pending to Running.
// See https://book.kubebuilder.io/reference/envtest.html for more details.
func updateHeadPodToRunningAndReady(ctx context.Context, rayClusterName string) {
	headPods := corev1.PodList{}
	headFilterLabels := client.MatchingLabels{
		utils.RayClusterLabelKey:  rayClusterName,
		utils.RayNodeTypeLabelKey: string(rayv1.HeadNode),
	}

	gomega.Eventually(
		listResourceFunc(ctx, &headPods, headFilterLabels, &client.ListOptions{Namespace: "default"}),
		time.Second*15, time.Millisecond*500).Should(gomega.Equal(1), "Head pod list should have only 1 Pod = %v", headPods.Items)

	headPod := headPods.Items[0]
	headPod.Status.Phase = corev1.PodRunning
	headPod.Status.Conditions = []corev1.PodCondition{
		{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		},
	}
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return k8sClient.Status().Update(ctx, &headPod)
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to update head Pod status to PodRunning")

	// Make sure the head Pod is updated.
	gomega.Eventually(
		isAllPodsRunning(ctx, headPods, headFilterLabels, "default"),
		time.Second*15, time.Millisecond*500).Should(gomega.BeTrue(), "Head Pod should be running: %v", headPods.Items)
}
