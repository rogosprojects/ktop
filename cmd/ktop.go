package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vladimirvivien/ktop/application"
	"github.com/vladimirvivien/ktop/k8s"
	"github.com/vladimirvivien/ktop/views/overview"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var (
	examples = `
# Start ktop using default configuration for the "default" namespace
%[1]s

# Start ktop with default configuration for all accessible namespaces
%[1]s -A

# Start ktop for a specific namespace in current context
%[1]s --namespace <namespace>

# Start ktop for a specific namespace and context
%[1]s --namespace <namespace> --context <context>

# Start ktop with custom refresh intervals (in seconds)
%[1]s --summary-refresh 10 --nodes-refresh 8 --pods-refresh 5
`
)

type ktopCmdOptions struct {
	namespace      string
	allNamespaces  bool
	context        string
	kubeconfig     string
	kubeFlags      *genericclioptions.ConfigFlags
	page           string // future use
	nodeColumns    string // comma-separated list of node columns to display
	podColumns     string // comma-separated list of pod columns to display
	showAllColumns bool   // show all columns
	summaryRefresh int    // summary stats refresh interval in seconds
	nodesRefresh   int    // nodes stats refresh interval in seconds
	podsRefresh    int    // pods stats refresh interval in seconds
}

// NewKtopCmd returns a command for ktop
func NewKtopCmd() *cobra.Command {
	o := &ktopCmdOptions{kubeFlags: genericclioptions.NewConfigFlags(false)}
	program := filepath.Base(os.Args[0])
	pluginMode := strings.HasPrefix(program, "kubectl-")
	usage := fmt.Sprintf("%s [flags]", program)
	shortDesc := fmt.Sprintf("Runs %s (standalone)", program)
	if pluginMode {
		shortDesc = fmt.Sprintf("Runs %s as kubectl plugin", program)
	}

	cmd := &cobra.Command{
		Use:          usage,
		Short:        shortDesc,
		Example:      fmt.Sprintf(examples, program),
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			return o.runKtop(c, args)
		},
	}
	cmd.Flags().BoolVarP(&o.allNamespaces, "all-namespaces", "A", false, "If true, display metrics for all accessible namespaces")
	cmd.Flags().StringVar(&o.nodeColumns, "node-columns", "", "Comma-separated list of node columns to display (e.g. 'NAME,CPU,MEM')")
	cmd.Flags().StringVar(&o.podColumns, "pod-columns", "", "Comma-separated list of pod columns to display (e.g. 'NAMESPACE,POD,STATUS')")
	cmd.Flags().BoolVar(&o.showAllColumns, "show-all-columns", true, "If true, show all columns (default)")
	cmd.Flags().IntVar(&o.summaryRefresh, "summary-refresh", 5, "Refresh interval for summary stats in seconds (default 5)")
	cmd.Flags().IntVar(&o.nodesRefresh, "nodes-refresh", 5, "Refresh interval for node stats in seconds (default 5)")
	cmd.Flags().IntVar(&o.podsRefresh, "pods-refresh", 3, "Refresh interval for pod stats in seconds (default 3)")
	o.kubeFlags.AddFlags(cmd.Flags())
	return cmd
}

func (o *ktopCmdOptions) runKtop(c *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if o.allNamespaces {
		o.namespace = k8s.AllNamespaces
	}

	k8sC, err := k8s.New(o.kubeFlags)
	if err != nil {
		return fmt.Errorf("ktop: failed to create Kubernetes client: %s", err)
	}
	fmt.Printf("Connected to: %s\n", k8sC.RESTConfig().Host)

	// Set refresh intervals
	k8sController := k8sC.Controller()
	k8sController.SummaryRefreshInterval = time.Duration(o.summaryRefresh) * time.Second
	k8sController.NodesRefreshInterval = time.Duration(o.nodesRefresh) * time.Second
	k8sController.PodsRefreshInterval = time.Duration(o.podsRefresh) * time.Second

	app := application.New(k8sC)
	app.WelcomeBanner()

	// Process column options
	nodeColumns := []string{}
	if o.nodeColumns != "" {
		nodeColumns = strings.Split(o.nodeColumns, ",")
		o.showAllColumns = false
	}

	podColumns := []string{}
	if o.podColumns != "" {
		podColumns = strings.Split(o.podColumns, ",")
		o.showAllColumns = false
	}

	// Create a new overview page with column options
	app.AddPage(overview.NewWithColumnOptions(app, "Overview", o.showAllColumns, nodeColumns, podColumns))

	if err := k8sC.AssertCoreAuthz(ctx); err != nil {
		return fmt.Errorf("ktop: %s", err)
	}

	// launch application
	appErr := make(chan error)
	go func() {
		appErr <- app.Run(ctx)
	}()

	select {
	case err := <-appErr:
		if err != nil {
			fmt.Printf("app error: %s\n", err)
			os.Exit(1)
		}
	case <-ctx.Done():
	}

	return nil
}
