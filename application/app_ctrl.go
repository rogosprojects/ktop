package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vladimirvivien/ktop/buildinfo"

	"github.com/vladimirvivien/ktop/k8s"
	"github.com/vladimirvivien/ktop/ui"
	"github.com/vladimirvivien/ktop/views/model"
)

type AppPage struct {
	Title string
	Panel ui.PanelController
}

type Application struct {
	namespace   string
	k8sClient   *k8s.Client
	tviewApp    *tview.Application
	pages       []AppPage
	modals      []tview.Primitive
	pageIdx     int
	tabIdx      int
	visibleView int
	panel       *appPanel
	refreshQ    chan struct{}
	stopCh      chan struct{}
}

func New(k8sC *k8s.Client) *Application {
	tapp := tview.NewApplication()
	app := &Application{
		k8sClient: k8sC,
		namespace: k8sC.Namespace(),
		tviewApp:  tapp,
		panel:     newPanel(tapp),
		refreshQ:  make(chan struct{}, 1),
		pageIdx:   -1,
		tabIdx:    -1,
	}
	return app
}

func (app *Application) GetK8sClient() *k8s.Client {
	return app.k8sClient
}

func (app *Application) AddPage(panel ui.PanelController) {
	app.pages = append(app.pages, AppPage{Title: panel.GetTitle(), Panel: panel})
}

func (app *Application) ShowModal(view tview.Primitive) {
	app.panel.showModalView(view)
}

func (app *Application) Focus(t tview.Primitive) {
	app.tviewApp.SetFocus(t)
}

func (app *Application) Refresh() {
	// Use a timeout to ensure we don't block if the channel is full
	select {
	case app.refreshQ <- struct{}{}:
		// Refresh message sent
	case <-time.After(100 * time.Millisecond):
		// Refresh timed out - channel might be full
	}
}

func (app *Application) ShowPanel(i int) {
	app.visibleView = i
}

func (app *Application) GetStopChan() <-chan struct{} {
	return app.stopCh
}

func (app *Application) WelcomeBanner() {
	fmt.Println(`
 _    _ 
| | _| |_ ___  _ __
| |/ / __/ _ \| '_ \
|   <| || (_) | |_) |
|_|\_\\__\___/| .__/
              |_|`)
	fmt.Printf("Version %s \n", buildinfo.Version)
}

func (app *Application) setup(ctx context.Context) error {
	// setup each page panel
	for _, page := range app.pages {
		if err := page.Panel.Run(ctx); err != nil {
			return fmt.Errorf("init failed: page %s: %s", page.Title, err)
		}
	}

	// continue setup rest of UI
	app.panel.Layout(app.pages)

	var hdr strings.Builder
	hdr.WriteString("%c [green]API server: [white]%s [green]Version: [white]%s [green]context: [white]%s [green]User: [white]%s [green]namespace: [white]%s [green] metrics:")
	if err := app.GetK8sClient().AssertMetricsAvailable(); err != nil {
		hdr.WriteString(" [red]not connected")
	} else {
		hdr.WriteString(" [white]connected")
	}

	namespace := app.k8sClient.Namespace()
	if namespace == k8s.AllNamespaces {
		namespace = "[orange](all)"
	}
	client := app.GetK8sClient()
	app.panel.DrawHeader(fmt.Sprintf(
		hdr.String(),
		ui.Icons.Rocket, client.RESTConfig().Host, client.GetServerVersion(), client.ClusterContext(), client.Username(), namespace,
	))

	app.panel.DrawFooter(app.getPageTitles()[app.visibleView])

	app.tviewApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Key press handling
		
		// Try to handle both Shift+key and uppercase key (in case shift handling is problematic)
		handleSortKey := false
		
		// Check for uppercase letters (alternative to Shift+key)
		isUppercaseKey := false
		if event.Key() == tcell.KeyRune {
			r := event.Rune()
			if r >= 'A' && r <= 'Z' {
				isUppercaseKey = true
			}
		}
		
		// Handle uppercase letters or explicit Shift+key combos
		if (isUppercaseKey || event.Modifiers()&tcell.ModShift != 0) && app.visibleView == 0 {
			handleSortKey = true
		}
		
		if handleSortKey {
			// Map keys to sort fields
			var sortField string
			switch event.Key() {
			case tcell.KeyRune:
				r := event.Rune()
				// Convert uppercase to lowercase for checking
				if r >= 'A' && r <= 'Z' {
					r = r + ('a' - 'A')
				}
				
				switch r {
				case 'n': // Namespace
					sortField = "NAMESPACE"
				case 'p': // Pod name
					sortField = "POD"
				case 's': // Status
					sortField = "STATUS"
				case 'a': // Age
					sortField = "AGE"
				case 'o': // Node
					sortField = "NODE"
				case 'r': // Ready
					sortField = "READY"
				case 't': // Restarts
					sortField = "RESTARTS"
				case 'c': // CPU
					sortField = "CPU"
				case 'm': // Memory
					sortField = "MEMORY"
				case 'i': // IP
					sortField = "IP"
				case 'v': // Volumes
					sortField = "VOLS"
				default:
					// Not a sort key, continue with normal event handling
					break
				}

				if sortField != "" {
					// Sort key detected
					
					// Update the sort field and trigger refresh
					model.SetSortField(model.SortField(sortField))
					
					// Trigger pod refresh with our new sort order
					app.k8sClient.Controller().TriggerPodRefresh()
					
					// Also refresh the UI to make sure everything is updated
					app.Refresh()
					
					return nil
				}
			}
		}

		if event.Key() == tcell.KeyEsc {
			app.Stop()
		}

		if event.Key() == tcell.KeyTAB {
			views := app.pages[0].Panel.GetChildrenViews()
			app.tabIdx++
			app.Focus(views[app.tabIdx])
			if app.tabIdx == len(views)-1 {
				app.tabIdx = -1
			}
		}

		if event.Key() < tcell.KeyF1 || event.Key() > tcell.KeyF12 {
			return event
		}

		keyPos := event.Key() - tcell.KeyF1
		titles := app.getPageTitles()
		if (keyPos >= 0 || keyPos <= 9) && (int(keyPos) <= len(titles)-1) {
			app.panel.switchToPage(app.getPageTitles()[keyPos])
		}

		return event
	})

	return nil
}

func (app *Application) Run(ctx context.Context) error {

	// setup application UI
	if err := app.setup(ctx); err != nil {
		return err
	}

	// setup refresh queue
	go func() {
		for range app.refreshQ {
			app.tviewApp.Draw()
		}
	}()

	return app.tviewApp.Run()
}

func (app *Application) Stop() error {
	if app.tviewApp == nil {
		return errors.New("failed to stop, tview.Application nil")
	}
	app.tviewApp.Stop()
	fmt.Println("ktop finished")
	return nil
}

func (app *Application) getPageTitles() (titles []string) {
	for _, page := range app.pages {
		titles = append(titles, page.Title)
	}
	return
}
