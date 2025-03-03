package overview

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vladimirvivien/ktop/application"
	"github.com/vladimirvivien/ktop/ui"
	"github.com/vladimirvivien/ktop/views/model"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/duration"
)

type clusterSummaryPanel struct {
	app          *application.Application
	title        string
	root         *tview.Flex
	children     []tview.Primitive
	listCols     []string
	graphTable   *tview.Table
	summaryTable *tview.Table
}

func NewClusterSummaryPanel(app *application.Application, title string) ui.Panel {
	p := &clusterSummaryPanel{app: app, title: title}
	p.Layout(nil)
	p.children = append(p.children, p.graphTable)
	return p
}

func (p *clusterSummaryPanel) GetTitle() string {
	return p.title
}
func (p *clusterSummaryPanel) Layout(data interface{}) {
	p.summaryTable = tview.NewTable()
	p.summaryTable.SetBorder(false)
	p.summaryTable.SetBorders(false)
	p.summaryTable.SetTitleAlign(tview.AlignLeft)
	p.summaryTable.SetBorderColor(tcell.ColorWhite)

	p.graphTable = tview.NewTable()
	p.graphTable.SetBorder(false)
	p.graphTable.SetBorders(false)
	p.graphTable.SetTitleAlign(tview.AlignLeft)
	p.graphTable.SetBorderColor(tcell.ColorWhite)

	refreshTime := p.app.GetK8sClient().Controller().SummaryRefreshInterval.Seconds()
	title := fmt.Sprintf("%s [gray](refresh: %.0fs)[white]", p.GetTitle(), refreshTime)
	
	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(p.summaryTable, 1, 1, true).
		AddItem(p.graphTable, 1, 1, true)
	root.SetBorder(true)
	root.SetTitle(title)
	root.SetTitleAlign(tview.AlignLeft)
	root.SetBorderPadding(0, 0, 0, 0)
	p.root = root
}

func (p *clusterSummaryPanel) DrawHeader(data interface{}) {}

func (p *clusterSummaryPanel) DrawBody(data interface{}) {
	colorKeys := ui.ColorKeys{0: "green", 40: "yellow", 80: "red"}
	client := p.app.GetK8sClient()
	graphSize := 40
	switch summary := data.(type) {
	case model.ClusterSummary:
		var cpuRatio, memRatio ui.Ratio
		var cpuGraph, memGraph string
		var cpuMetrics, memMetrics string
		if err := client.AssertMetricsAvailable(); err != nil { // metrics not available
			cpuRatio = ui.GetRatio(float64(summary.RequestedPodCpuTotal.MilliValue()), float64(summary.AllocatableNodeCpuTotal.MilliValue()))
			cpuGraph = ui.BarGraph(graphSize, cpuRatio, colorKeys)
			cpuMetrics = fmt.Sprintf(
				"CPU: [white][%s[white]] %dm/%dm (%02.1f%% requested)",
				cpuGraph, summary.RequestedPodCpuTotal.MilliValue(), summary.AllocatableNodeCpuTotal.MilliValue(), cpuRatio*100,
			)

			memRatio = ui.GetRatio(float64(summary.RequestedPodMemTotal.MilliValue()), float64(summary.AllocatableNodeMemTotal.MilliValue()))
			memGraph = ui.BarGraph(graphSize, memRatio, colorKeys)
			memMetrics = fmt.Sprintf(
				"Memory: [white][%s[white]] %dGi/%dGi (%02.1f%% requested)",
				memGraph, summary.RequestedPodMemTotal.ScaledValue(resource.Giga), summary.AllocatableNodeMemTotal.ScaledValue(resource.Giga), memRatio*100,
			)
		} else {
			cpuRatio = ui.GetRatio(float64(summary.UsageNodeCpuTotal.MilliValue()), float64(summary.AllocatableNodeCpuTotal.MilliValue()))
			cpuGraph = ui.BarGraph(graphSize, cpuRatio, colorKeys)
			cpuMetrics = fmt.Sprintf(
				"CPU: [white][%s[white]] %dm/%dm (%02.1f%% used)",
				cpuGraph, summary.UsageNodeCpuTotal.MilliValue(), summary.AllocatableNodeCpuTotal.MilliValue(), cpuRatio*100,
			)

			memRatio = ui.GetRatio(float64(summary.UsageNodeMemTotal.MilliValue()), float64(summary.AllocatableNodeMemTotal.MilliValue()))
			memGraph = ui.BarGraph(graphSize, memRatio, colorKeys)
			memMetrics = fmt.Sprintf(
				"Memory: [white][%s[white]] %dGi/%dGi (%02.1f%% used)",
				memGraph, summary.UsageNodeMemTotal.ScaledValue(resource.Giga), summary.AllocatableNodeMemTotal.ScaledValue(resource.Giga), memRatio*100,
			)
		}

		p.graphTable.SetCell(
			0, 0,
			tview.NewTableCell(cpuMetrics).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignLeft).
				SetExpansion(100),
		)

		p.graphTable.SetCell(
			0, 1,
			tview.NewTableCell(memMetrics).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignLeft).
				SetExpansion(100),
		)

		// -=-=-=-=-=-=-=-=-=-=-=-=- cluster summary table -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-
		p.summaryTable.SetCell(
			0, 0,
			tview.NewTableCell(fmt.Sprintf("Uptime: [white]%s[white]", duration.HumanDuration(time.Since(summary.Uptime.Time)))).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignLeft).
				SetExpansion(100),
		)
		p.summaryTable.SetCell(
			0, 1,
			tview.NewTableCell(fmt.Sprintf("Nodes: [white]%d", summary.NodesReady)).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignLeft).
				SetExpansion(100),
		)
		p.summaryTable.SetCell(
			0, 2,
			tview.NewTableCell(fmt.Sprintf("Namespaces: [white]%d[white]", summary.Namespaces)).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignLeft).
				SetExpansion(100),
		)

		p.summaryTable.SetCell(
			0, 3,
			tview.NewTableCell(fmt.Sprintf("Pods: [white]%d/%d (%d imgs)", summary.PodsRunning, summary.PodsAvailable, summary.ImagesCount)).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignLeft).
				SetExpansion(100),
		)

		p.summaryTable.SetCell(
			0, 5,
			tview.NewTableCell(fmt.Sprintf("Deployments: [white]%d/%d", summary.DeploymentsReady, summary.DeploymentsTotal)).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignLeft).
				SetExpansion(100),
		)

		p.summaryTable.SetCell(
			0, 6,
			tview.NewTableCell(fmt.Sprintf("Sets: [white]replicas %d, daemons %d, stateful %d", summary.ReplicaSetsReady, summary.DaemonSetsReady, summary.StatefulSetsReady)).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignLeft).
				SetExpansion(100),
		)

		p.summaryTable.SetCell(
			0, 9,
			tview.NewTableCell(fmt.Sprintf("Jobs: [white]%d (cron: %d)", summary.JobsCount, summary.CronJobsCount)).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignLeft).
				SetExpansion(100),
		)

		p.summaryTable.SetCell(
			0, 10,
			tview.NewTableCell(fmt.Sprintf(
				"[yellow]PVs: [white]%d (%dGi) [yellow]PVCs: [white]%d (%dGi)",
				summary.PVCCount, summary.PVsTotal.ScaledValue(resource.Giga),
				summary.PVCCount, summary.PVCsTotal.ScaledValue(resource.Giga),
			)).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignLeft).
				SetExpansion(100),
		)
	default:
		panic(fmt.Sprintf("SummaryPanel.DrawBody: unexpected type %T", data))
	}
}

func (p *clusterSummaryPanel) DrawFooter(data interface{}) {}

func (p *clusterSummaryPanel) Clear() {}

func (p *clusterSummaryPanel) GetRootView() tview.Primitive {
	return p.root
}

func (p *clusterSummaryPanel) GetChildrenViews() []tview.Primitive {
	return p.children
}
