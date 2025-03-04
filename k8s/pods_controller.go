package k8s

import (
	"context"
	"time"

	"github.com/vladimirvivien/ktop/views/model"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	metricsV1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func (c *Controller) GetPodList(ctx context.Context) ([]*coreV1.Pod, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	items, err := c.podInformer.Lister().List(labels.Everything())
	if err != nil {
		return nil, err
	}
	return items, nil
}

// Helper function to calculate total CPU and memory usage from pod metrics
func podMetricsTotals(metrics *metricsV1beta1.PodMetrics) (totalCpu, totalMem *resource.Quantity) {
	totalCpu = resource.NewQuantity(0, resource.DecimalSI)
	totalMem = resource.NewQuantity(0, resource.DecimalSI)
	for _, c := range metrics.Containers {
		totalCpu.Add(*c.Usage.Cpu())
		totalMem.Add(*c.Usage.Memory())
	}
	return
}

func (c *Controller) GetPodModels(ctx context.Context) (models []model.PodModel, err error) {
	pods, err := c.GetPodList(ctx)
	if err != nil {
		return
	}
	nodeMetricsCache := make(map[string]*metricsV1beta1.NodeMetrics)
	nodeAllocResMap := make(map[string]coreV1.ResourceList)
	for _, pod := range pods {

		// retrieve metrics per pod
		podMetrics, err := c.GetPodMetricsByName(ctx, pod)
		if err != nil {
			podMetrics = new(metricsV1beta1.PodMetrics)
		}

		// retrieve and cache node metrics for related pod-node
		if metrics, ok := nodeMetricsCache[pod.Spec.NodeName]; !ok {
			metrics, err = c.GetNodeMetrics(ctx, pod.Spec.NodeName)
			if err != nil {
				metrics = new(metricsV1beta1.NodeMetrics)
			}
			nodeMetricsCache[pod.Spec.NodeName] = metrics
		}
		nodeMetrics := nodeMetricsCache[pod.Spec.NodeName]

		model := model.NewPodModel(pod, podMetrics, nodeMetrics)

		// Track pod peak metrics
		podKey := pod.Namespace + "/" + pod.Name

		if podMetrics.Containers != nil && len(podMetrics.Containers) > 0 {
			// Get totals for CPU and memory
			totalCpu, totalMem := podMetricsTotals(podMetrics)

			// Initialize peak tracking for this pod if needed
			if _, exists := c.PeakPodCPU[podKey]; !exists {
				c.PeakPodCPU[podKey] = resource.NewQuantity(0, resource.DecimalSI)
			}
			if _, exists := c.PeakPodMemory[podKey]; !exists {
				c.PeakPodMemory[podKey] = resource.NewQuantity(0, resource.DecimalSI)
			}

			// Update peaks if current usage is higher
			if totalCpu.Cmp(*c.PeakPodCPU[podKey]) > 0 {
				cpuCopy := totalCpu.DeepCopy()
				c.PeakPodCPU[podKey] = &cpuCopy
			}
			if totalMem.Cmp(*c.PeakPodMemory[podKey]) > 0 {
				memCopy := totalMem.DeepCopy()
				c.PeakPodMemory[podKey] = &memCopy
			}
		}

		// retrieve pod's node allocatable resources
		if alloc, ok := nodeAllocResMap[pod.Spec.NodeName]; !ok {
			node, err := c.GetNode(ctx, pod.Spec.NodeName)
			if err != nil {
				alloc = coreV1.ResourceList{}
			} else {
				alloc = node.Status.Allocatable
			}
			nodeAllocResMap[pod.Spec.NodeName] = alloc
		}
		alloc := nodeAllocResMap[pod.Spec.NodeName]
		model.NodeAllocatableMemQty = alloc.Memory()
		model.NodeAllocatableCpuQty = alloc.Cpu()
		models = append(models, *model)
	}
	return
}

func (c *Controller) installPodsHandler(ctx context.Context, refreshFunc RefreshPodsFunc) {
	if refreshFunc == nil {
		return
	}
	go func() {
		c.refreshPods(ctx, refreshFunc) // initial refresh
		ticker := time.NewTicker(c.PodsRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.refreshPods(ctx, refreshFunc); err != nil {
					continue
				}
			}
		}
	}()
}

func (c *Controller) refreshPods(ctx context.Context, refreshFunc RefreshPodsFunc) error {
	models, err := c.GetPodModels(ctx)
	if err != nil {
		// Check if this is a context timeout error
		if ctx.Err() == context.DeadlineExceeded {
			// If we got some models despite timeout, use them for partial refresh
			if len(models) > 0 {
				refreshFunc(ctx, models)
			}
		}
		return err
	}
	refreshFunc(ctx, models)
	return nil
}
