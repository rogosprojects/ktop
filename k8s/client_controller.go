package k8s

import (
	"context"
	"errors"
	"time"

	"github.com/vladimirvivien/ktop/views/model"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/informers"
	appsV1Informers "k8s.io/client-go/informers/apps/v1"
	batchV1Informers "k8s.io/client-go/informers/batch/v1"
	coreV1Informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type RefreshNodesFunc func(ctx context.Context, items []model.NodeModel) error
type RefreshPodsFunc func(ctx context.Context, items []model.PodModel) error
type RefreshSummaryFunc func(ctx context.Context, items model.ClusterSummary) error

type Controller struct {
	client *Client

	nodeMetricsInformer *NodeMetricsInformer
	podMetricsInformer  *PodMetricsInformer
	namespaceInformer   coreV1Informers.NamespaceInformer
	nodeInformer        coreV1Informers.NodeInformer
	podInformer         coreV1Informers.PodInformer
	pvInformer          coreV1Informers.PersistentVolumeInformer
	pvcInformer         coreV1Informers.PersistentVolumeClaimInformer

	jobInformer     batchV1Informers.JobInformer
	cronJobInformer batchV1Informers.CronJobInformer

	deploymentInformer  appsV1Informers.DeploymentInformer
	daemonSetInformer   appsV1Informers.DaemonSetInformer
	replicaSetInformer  appsV1Informers.ReplicaSetInformer
	statefulSetInformer appsV1Informers.StatefulSetInformer

	nodeRefreshFunc    RefreshNodesFunc
	podRefreshFunc     RefreshPodsFunc
	summaryRefreshFunc RefreshSummaryFunc

	// Refresh intervals
	SummaryRefreshInterval time.Duration
	NodesRefreshInterval   time.Duration
	PodsRefreshInterval    time.Duration

	// Peak metrics tracking
	PeakNodeCPU      map[string]*resource.Quantity // map of node name to peak CPU
	PeakNodeMemory   map[string]*resource.Quantity // map of node name to peak Memory
	PeakPodCPU       map[string]*resource.Quantity // map of pod key to peak CPU
	PeakPodMemory    map[string]*resource.Quantity // map of pod key to peak Memory
	PeakClusterCPU   *resource.Quantity            // peak cluster CPU usage
	PeakClusterMemory *resource.Quantity           // peak cluster Memory usage
}

func newController(client *Client) *Controller {
	ctrl := &Controller{
		client:                 client,
		SummaryRefreshInterval: 5 * time.Second,
		NodesRefreshInterval:   5 * time.Second,
		PodsRefreshInterval:    3 * time.Second,
		PeakNodeCPU:            make(map[string]*resource.Quantity),
		PeakNodeMemory:         make(map[string]*resource.Quantity),
		PeakPodCPU:             make(map[string]*resource.Quantity),
		PeakPodMemory:          make(map[string]*resource.Quantity),
		PeakClusterCPU:         resource.NewQuantity(0, resource.DecimalSI),
		PeakClusterMemory:      resource.NewQuantity(0, resource.DecimalSI),
	}
	return ctrl
}

func (c *Controller) SetNodeRefreshFunc(fn RefreshNodesFunc) *Controller {
	c.nodeRefreshFunc = fn
	return c
}
func (c *Controller) SetPodRefreshFunc(fn RefreshPodsFunc) *Controller {
	c.podRefreshFunc = fn
	return c
}

func (c *Controller) SetClusterSummaryRefreshFunc(fn RefreshSummaryFunc) *Controller {
	c.summaryRefreshFunc = fn
	return c
}

// GetCurrentPodModels returns the current pod models for sorting/display
// This is used when manually refreshing the pod display
func (c *Controller) GetCurrentPodModels() []model.PodModel {
	// Get a new context for this operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Get the models
	models, err := c.GetPodModels(ctx)
	if err != nil {
		// Return empty slice on error
		return []model.PodModel{}
	}
	
	return models
}

// TriggerPodRefresh manually triggers the pod refresh function
// This is used when sorting pods
func (c *Controller) TriggerPodRefresh() {
	if c.podRefreshFunc == nil {
		return
	}
	
	// Create a context for the operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Call refreshPods to get the latest data and update the display
	c.refreshPods(ctx, c.podRefreshFunc)
}

func (c *Controller) Start(ctx context.Context, resync time.Duration) error {
	if ctx == nil {
		return errors.New("context cannot be nil")
	}

	// initialize

	if err := c.client.AssertMetricsAvailable(); err == nil {
		c.nodeMetricsInformer = NewNodeMetricsInformer(c.client.metricsClient, resync)
		nodeMetricsInformerHasSynced := c.nodeMetricsInformer.Informer().HasSynced

		c.podMetricsInformer = NewPodMetricsInformer(c.client.metricsClient, resync, c.client.namespace)
		podMetricsInformerHasSynced := c.podMetricsInformer.Informer().HasSynced

		go c.nodeMetricsInformer.Informer().Run(ctx.Done())
		go c.podMetricsInformer.Informer().Run(ctx.Done())

		if ok := cache.WaitForCacheSync(ctx.Done(), nodeMetricsInformerHasSynced, podMetricsInformerHasSynced); !ok {
			panic("metrics resources failed to sync [nodes, pods, containers]")
		}

	}

	// initialize informer factories
	var factory informers.SharedInformerFactory
	if c.client.namespace == AllNamespaces {
		factory = informers.NewSharedInformerFactory(c.client.kubeClient, resync)
	} else {
		factory = informers.NewSharedInformerFactoryWithOptions(c.client.kubeClient, resync, informers.WithNamespace(c.client.namespace))
	}

	// NOTE: the followings captures each informer
	// and also calls Informer() method to register the cached type.
	// Call to Informer() must happen before factory.Star() or it hangs.

	// core/V1 informers
	coreInformers := factory.Core().V1()
	c.namespaceInformer = coreInformers.Namespaces()
	namespaceHasSynced := c.namespaceInformer.Informer().HasSynced
	c.nodeInformer = coreInformers.Nodes()
	nodeHasSynced := c.nodeInformer.Informer().HasSynced
	c.podInformer = coreInformers.Pods()
	podHasSynced := c.podInformer.Informer().HasSynced
	c.pvInformer = coreInformers.PersistentVolumes()
	pvHasSynced := c.pvInformer.Informer().HasSynced
	c.pvcInformer = coreInformers.PersistentVolumeClaims()
	pvcHasSynced := c.pvcInformer.Informer().HasSynced

	// Apps/v1 Informers
	appsInformers := factory.Apps().V1()
	c.deploymentInformer = appsInformers.Deployments()
	deploymentHasSynced := c.deploymentInformer.Informer().HasSynced
	c.daemonSetInformer = appsInformers.DaemonSets()
	daemonsetHasSynced := c.daemonSetInformer.Informer().HasSynced
	c.replicaSetInformer = appsInformers.ReplicaSets()
	replicasetHasSynced := c.replicaSetInformer.Informer().HasSynced
	c.statefulSetInformer = appsInformers.StatefulSets()
	statefulsetHasSynced := c.statefulSetInformer.Informer().HasSynced

	// Batch informers
	batchInformers := factory.Batch().V1()
	c.jobInformer = batchInformers.Jobs()
	jobHasSynced := c.jobInformer.Informer().HasSynced
	c.cronJobInformer = batchInformers.CronJobs()
	cronJobHasSynced := c.cronJobInformer.Informer().HasSynced

	factory.Start(ctx.Done())

	// wait immediately for core resources to syn
	// wait for core resources to sync
	if ok := cache.WaitForCacheSync(ctx.Done(),
		namespaceHasSynced,
		nodeHasSynced,
		podHasSynced,
	); !ok {
		panic("core resources failed to sync [namespaces, nodes, pods]")
	}

	// defer waiting for non-core resources to sync
	go func() {
		ok := cache.WaitForCacheSync(ctx.Done(),
			pvHasSynced,
			pvcHasSynced,
			deploymentHasSynced,
			daemonsetHasSynced,
			replicasetHasSynced,
			statefulsetHasSynced,
			jobHasSynced,
			cronJobHasSynced,
		)
		if !ok {
			panic("resource failed to sync")
		}
	}()

	c.setupSummaryHandler(ctx, c.summaryRefreshFunc)
	c.setupNodeHandler(ctx, c.nodeRefreshFunc)
	c.installPodsHandler(ctx, c.podRefreshFunc)

	return nil
}
