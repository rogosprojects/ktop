package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	// Use a non-blocking send to prevent UI deadlocks
	// If channel is full, we'll drop this refresh and let the next scheduled refresh happen
	select {
	case app.refreshQ <- struct{}{}:
		// Refresh message sent successfully
	default:
		// Channel is full, log or handle if needed
		// This is intentionally non-blocking to prevent UI freezes
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
		
		// Ignore any key events with Command/Ctrl/Alt modifiers to prevent panel disappearing
		if event.Modifiers()&(tcell.ModCtrl|tcell.ModMeta|tcell.ModAlt) != 0 {
			return event
		}

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
		// Only process if it's a pure Shift key or uppercase letter
		modIsOnlyShift := event.Modifiers() == tcell.ModShift
		if (isUppercaseKey || modIsOnlyShift) && app.visibleView == 0 {
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

					// Store the current title to maintain page visibility
					currentTitle := app.getPageTitles()[app.visibleView]
					
					// Trigger pod refresh with our new sort order and handle any errors
					err := app.k8sClient.Controller().TriggerPodRefresh()
					if err != nil {
						// Even if there's an error, we still want to refresh
						// with whatever data we have so far
						// But we don't change the page/panel
						app.panel.DrawFooter(currentTitle)
					} else {
						// Keep the same page visible - don't switch pages
						app.panel.DrawFooter(currentTitle)
					}

					// Also refresh the UI to make sure everything is updated
					app.Refresh()
					
					// Show sorting info in footer instead of changing pages
					app.panel.showSortInfo(fmt.Sprintf("Sort by %s", sortField))

					return nil
				}
			}
		}

		if event.Key() == tcell.KeyEsc {
			app.Stop()
		}

		if event.Key() == tcell.KeyTAB {
			// Since GetChildrenViews now only returns the pod panel,
			// we can simply get the first item in the views list
			views := app.pages[0].Panel.GetChildrenViews()
			if len(views) > 0 {
				// Focus on the first (and only) item - the pod panel
				app.Focus(views[0])
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
