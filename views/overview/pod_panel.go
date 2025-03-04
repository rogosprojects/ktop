package overview

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vladimirvivien/ktop/application"
	"github.com/vladimirvivien/ktop/ui"
	"github.com/vladimirvivien/ktop/views/model"
	"k8s.io/apimachinery/pkg/api/resource"
)

type podPanel struct {
	app      *application.Application
	title    string
	root     *tview.Flex
	children []tview.Primitive
	listCols []string
	list     *tview.Table
	laidout  bool
	colMap   map[string]int // Maps column name to position index
}

func NewPodPanel(app *application.Application, title string) ui.Panel {
	p := &podPanel{app: app, title: title}
	p.Layout(nil)

	return p
}

func (p *podPanel) GetTitle() string {
	return p.title
}

func (p *podPanel) Layout(_ interface{}) {
	if !p.laidout {
		p.list = tview.NewTable()
		p.list.SetFixed(1, 0)
		p.list.SetBorder(false)
		p.list.SetBorders(false)
		p.list.SetFocusFunc(func() {
			p.list.SetSelectable(true, false)
			p.list.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorYellow).Foreground(tcell.ColorBlue))
		})
		p.list.SetBlurFunc(func() {
			p.list.SetSelectable(false, false)
		})

		p.root = tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(p.list, 0, 1, true)
		p.root.SetBorder(true)
		p.root.SetTitle(p.GetTitle())
		p.root.SetTitleAlign(tview.AlignLeft)
		p.laidout = true
		
		// Add the footer
		p.DrawFooter(nil)
	}
}

func (p *podPanel) DrawHeader(data interface{}) {
	cols, ok := data.([]string)
	if !ok {
		panic(fmt.Sprintf("podPanel.DrawBody got unexpected data type %T", data))
	}

	// Initialize the column map
	p.colMap = make(map[string]int)
	p.listCols = cols
	
	// Get the current sort field to highlight it
	currentSortField := model.GetCurrentSortField()
	sortDir := model.GetCurrentSortDirection()
	
	// Set column headers and build column map
	for i, col := range p.listCols {
		// Determine if this column is the one being sorted
		isSortedCol := string(currentSortField) == col
		
		// Create header text, adding sort indicator if this is the sorted column
		headerText := col
		if isSortedCol {
			if sortDir > 0 {
				headerText = col + " ↑" // Ascending
			} else {
				headerText = col + " ↓" // Descending
			}
		}
		
		// Set background color to highlight the sorted column
		bgColor := tcell.ColorDarkGreen
		if isSortedCol {
			bgColor = tcell.ColorDarkBlue // Highlight the sorted column
		}
		
		p.list.SetCell(0, i,
			tview.NewTableCell(headerText).
				SetTextColor(tcell.ColorWhite).
				SetBackgroundColor(bgColor).
				SetAlign(tview.AlignLeft).
				SetExpansion(100).
				SetSelectable(false),
		)
		
		// Map column name to position
		p.colMap[col] = i
	}
	p.list.SetFixed(1, 0)
}

func (p *podPanel) DrawBody(data interface{}) {
	pods, ok := data.([]model.PodModel)
	if !ok {
		panic(fmt.Sprintf("PodPanel.DrawBody got unexpected type %T", data))
	}

	client := p.app.GetK8sClient()
	metricsDisabled := client.AssertMetricsAvailable() != nil
	colorKeys := ui.ColorKeys{0: "green", 50: "yellow", 90: "red"}
	var cpuRatio, memRatio ui.Ratio
	var cpuGraph, memGraph string
	var cpuMetrics, memMetrics string

	refreshTime := p.app.GetK8sClient().Controller().PodsRefreshInterval.Seconds()
	
	// Get current sort field and direction for display
	sortField := model.GetCurrentSortField()
	sortDir := model.GetCurrentSortDirection()
	dirIndicator := "↑" // Ascending
	if sortDir < 0 {
		dirIndicator = "↓" // Descending
	}
	
	// Add sort info to the title
	p.root.SetTitle(fmt.Sprintf("%s(%d) [gray](refresh: %.0fs | sort: %s %s)[white]", 
		p.GetTitle(), len(pods), refreshTime, string(sortField), dirIndicator))
	p.root.SetTitleAlign(tview.AlignLeft)

	for rowIdx, pod := range pods {
		rowIdx++ // offset for header row
		
		// Render each column that is included in the filtered view
		for _, colName := range p.listCols {
			colIdx, exists := p.colMap[colName]
			if !exists {
				continue
			}
			
			switch colName {
			case "NAMESPACE":
				p.list.SetCell(
					rowIdx, colIdx,
					&tview.TableCell{
						Text:  pod.Namespace,
						Color: tcell.ColorYellow,
						Align: tview.AlignLeft,
					},
				)
				
			case "POD":
				p.list.SetCell(
					rowIdx, colIdx,
					&tview.TableCell{
						Text:  pod.Name,
						Color: tcell.ColorYellow,
						Align: tview.AlignLeft,
					},
				)
				
			case "READY":
				p.list.SetCell(
					rowIdx, colIdx,
					&tview.TableCell{
						Text:  fmt.Sprintf("%d/%d", pod.ReadyContainers, pod.TotalContainers),
						Color: tcell.ColorYellow,
						Align: tview.AlignLeft,
					},
				)
				
			case "STATUS":
				p.list.SetCell(
					rowIdx, colIdx,
					&tview.TableCell{
						Text:  pod.Status,
						Color: tcell.ColorYellow,
						Align: tview.AlignLeft,
					},
				)
				
			case "RESTARTS":
				p.list.SetCell(
					rowIdx, colIdx,
					&tview.TableCell{
						Text:  fmt.Sprintf("%d", pod.Restarts),
						Color: tcell.ColorYellow,
						Align: tview.AlignLeft,
					},
				)
				
			case "AGE":
				p.list.SetCell(
					rowIdx, colIdx,
					&tview.TableCell{
						Text:  pod.TimeSince,
						Color: tcell.ColorYellow,
						Align: tview.AlignLeft,
					},
				)
				
			case "VOLS":
				p.list.SetCell(
					rowIdx, colIdx,
					&tview.TableCell{
						Text:  fmt.Sprintf("%d", pod.Volumes),
						Color: tcell.ColorYellow,
						Align: tview.AlignLeft,
					},
				)
				
			case "IP":
				p.list.SetCell(
					rowIdx, colIdx,
					&tview.TableCell{
						Text:  pod.IP,
						Color: tcell.ColorYellow,
						Align: tview.AlignLeft,
					},
				)
				
			case "NODE":
				p.list.SetCell(
					rowIdx, colIdx,
					&tview.TableCell{
						Text:  pod.Node,
						Color: tcell.ColorYellow,
						Align: tview.AlignLeft,
					},
				)
				
			case "CPU":
				if metricsDisabled {
					// no CPU metrics
					p.list.SetCell(
						rowIdx, colIdx,
						&tview.TableCell{
							Text:  "unavailable",
							Color: tcell.ColorYellow,
							Align: tview.AlignLeft,
						},
					)
				} else {
					// Check if CPU limit is set (non-zero), otherwise use node limit
					var cpuDenominator float64
					var cpuLimitLabel string
					
					if pod.PodLimitCpuQty != nil && pod.PodLimitCpuQty.MilliValue() > 0 {
						// Use pod limit
						cpuDenominator = float64(pod.PodLimitCpuQty.MilliValue())
						cpuLimitLabel = fmt.Sprintf("%dm", pod.PodLimitCpuQty.MilliValue())
					} else {
						// Use node limit when pod limit is not set
						cpuDenominator = float64(pod.NodeAllocatableCpuQty.MilliValue())
						cpuLimitLabel = fmt.Sprintf("%dm*", pod.NodeAllocatableCpuQty.MilliValue())
					}
					
					cpuRatio = ui.GetRatio(float64(pod.PodUsageCpuQty.MilliValue()), cpuDenominator)
					cpuGraph = ui.BarGraph(10, cpuRatio, colorKeys)
					
					// Get peak CPU for this pod - show absolute value only
					podKey := pod.Namespace + "/" + pod.Name
					peakCPU, exists := client.Controller().PeakPodCPU[podKey]
					if exists && peakCPU != nil {
						cpuMetrics = fmt.Sprintf(
							"[white][%s[white]] %dm/%s (%1.0f%%) [gray](Peak: %dm)[white]",
							cpuGraph, pod.PodUsageCpuQty.MilliValue(), cpuLimitLabel, cpuRatio*100, peakCPU.MilliValue(),
						)
					} else {
						cpuMetrics = fmt.Sprintf(
							"[white][%s[white]] %dm/%s (%1.0f%%)",
							cpuGraph, pod.PodUsageCpuQty.MilliValue(), cpuLimitLabel, cpuRatio*100,
						)
					}
					
					p.list.SetCell(
						rowIdx, colIdx,
						&tview.TableCell{
							Text:  cpuMetrics,
							Color: tcell.ColorYellow,
							Align: tview.AlignLeft,
						},
					)
				}
				
			case "MEMORY":
				if metricsDisabled {
					// no Memory metrics
					p.list.SetCell(
						rowIdx, colIdx,
						&tview.TableCell{
							Text:  "unavailable",
							Color: tcell.ColorYellow,
							Align: tview.AlignLeft,
						},
					)
				} else {
					// Check if memory limit is set (non-zero), otherwise use node limit
					var memDenominator float64
					var memLimitLabel string
					var memLimitScaled int64
					
					if pod.PodLimitMemQty != nil && pod.PodLimitMemQty.Value() > 0 {
						// Use pod limit
						memDenominator = float64(pod.PodLimitMemQty.Value())
						memLimitScaled = pod.PodLimitMemQty.ScaledValue(resource.Mega)
						memLimitLabel = fmt.Sprintf("%dMi", memLimitScaled)
					} else {
						// Use node limit when pod limit is not set
						memDenominator = float64(pod.NodeAllocatableMemQty.Value())
						memLimitScaled = pod.NodeAllocatableMemQty.ScaledValue(resource.Mega)
						memLimitLabel = fmt.Sprintf("%dMi*", memLimitScaled)
					}
					
					memRatio = ui.GetRatio(float64(pod.PodUsageMemQty.Value()), memDenominator)
					memGraph = ui.BarGraph(10, memRatio, colorKeys)
					
					// Get peak Memory for this pod - show absolute value only
					podKey := pod.Namespace + "/" + pod.Name
					peakMem, exists := client.Controller().PeakPodMemory[podKey]
					if exists && peakMem != nil {
						memMetrics = fmt.Sprintf(
							"[white][%s[white]] %dMi/%s (%1.0f%%) [gray](Peak: %dMi)[white]",
							memGraph, 
							pod.PodUsageMemQty.ScaledValue(resource.Mega), 
							memLimitLabel, 
							memRatio*100,
							peakMem.ScaledValue(resource.Mega),
						)
					} else {
						memMetrics = fmt.Sprintf(
							"[white][%s[white]] %dMi/%s (%1.0f%%)",
							memGraph, 
							pod.PodUsageMemQty.ScaledValue(resource.Mega), 
							memLimitLabel, 
							memRatio*100,
						)
					}
					
					p.list.SetCell(
						rowIdx, colIdx,
						&tview.TableCell{
							Text:  memMetrics,
							Color: tcell.ColorYellow,
							Align: tview.AlignLeft,
						},
					)
				}
			}
		}
	}
}

func (p *podPanel) DrawFooter(_ interface{}) {
	// Add help text about column sorting in the footer
	footerText := "[gray]Sort: [white]Shift+N[gray](namespace) [white]Shift+P[gray](pod) [white]Shift+M[gray](memory) [white]Shift+C[gray](cpu) [white]Shift+A[gray](age) [white]Shift+S[gray](status) [white]Shift+O[gray](node)"
	
	// Create a text view for the footer
	footer := tview.NewTextView()
	footer.SetText(footerText)
	footer.SetTextAlign(tview.AlignLeft)
	footer.SetDynamicColors(true)
	
	// Add the footer to the root flex container
	if p.root != nil {
		// Remove any existing footer first
		for i := 0; i < p.root.GetItemCount(); i++ {
			// Skip the main list view
			if p.root.GetItem(i) == p.list {
				continue
			}
			// Remove any other items (should be the footer)
			p.root.RemoveItem(p.root.GetItem(i))
			break
		}
		
		// Add the new footer (using only 1 line)
		p.root.AddItem(footer, 1, 0, false)
	}
}

func (p *podPanel) Clear() {
	p.list.Clear()
	p.Layout(nil)
	p.DrawHeader(p.listCols)
	p.DrawFooter(nil) // Add the footer
}

func (p *podPanel) GetRootView() tview.Primitive {
	return p.root
}

func (p *podPanel) GetChildrenViews() []tview.Primitive {
	return p.children
}