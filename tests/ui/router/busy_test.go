package router_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"utils/internal/core/cleaner"
	corenetwork "utils/internal/core/network"
	"utils/internal/ui"
	"utils/internal/ui/views"
)

const canceledText = "Canceled. The run stopped early"

func TestRouterEscCancelsRunningCleanupAndShowsCanceledResult(t *testing.T) {
	run := newBlockingRun()
	d := newDriver(t, cleanerFeature(run.cleaner))

	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	run.waitStarted(t)

	d.send(tea.KeyMsg{Type: tea.KeyEsc})
	if view := d.view(); !strings.Contains(view, "Canceling cleanup") {
		t.Fatalf("expected the cleaner to stay open and show it is canceling:\n%s", view)
	}
	run.waitCanceled(t)
	close(run.release)

	d.pumpUntil(func() bool { return strings.Contains(d.view(), canceledText) })
	view := d.view()
	if !strings.Contains(view, "fake run stopped") || strings.Contains(view, "Completed with errors") {
		t.Fatalf("expected the canceled run's result, not an error summary:\n%s", view)
	}
}

func TestRouterEscCancelsRunningDiagnosticsAndShowsCanceledResult(t *testing.T) {
	run := newBlockingRun()
	d := newDriver(t, networkFeature(run))

	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	d.selectNetworkAction("Checks connectivity, DNS resolution")
	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	run.waitStarted(t)

	d.send(tea.KeyMsg{Type: tea.KeyEsc})
	run.waitCanceled(t)
	close(run.release)

	d.pumpUntil(func() bool { return strings.Contains(d.view(), canceledText) })
	if view := d.view(); strings.Contains(view, "Completed with errors") {
		t.Fatalf("expected canceled diagnostics to read as canceled:\n%s", view)
	}
}

func TestRouterCtrlCDuringRunCancelsAndQuitsOnlyAfterRunFinishes(t *testing.T) {
	run := newBlockingRun()
	d := newDriver(t, cleanerFeature(run.cleaner))

	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	run.waitStarted(t)

	d.send(tea.KeyMsg{Type: tea.KeyCtrlC})
	d.pumpFor(50 * time.Millisecond)
	if d.quit {
		t.Fatal("ctrl+c must not quit while the run is still stopping")
	}
	run.waitCanceled(t)

	close(run.release)
	d.pumpUntil(func() bool { return d.quit })
}

func TestRouterSecondCtrlCQuitsWithoutWaitingForRun(t *testing.T) {
	run := newBlockingRun()
	d := newDriver(t, cleanerFeature(run.cleaner))

	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	run.waitStarted(t)

	d.send(tea.KeyMsg{Type: tea.KeyCtrlC})
	d.send(tea.KeyMsg{Type: tea.KeyCtrlC})
	d.pumpUntil(func() bool { return d.quit })
	close(run.release)
}

func TestRouterEscAtCleanerPromptsReturnsToOptions(t *testing.T) {
	run := newBlockingRun()
	d := newDriver(t, cleanerFeature(run.cleaner))

	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	d.send(tea.KeyMsg{Type: tea.KeyEsc})
	if view := d.view(); !strings.Contains(view, "Options") || strings.Contains(view, "Choose cleanup mode") {
		t.Fatalf("expected esc at the mode prompt to return to the cleaner options:\n%s", view)
	}

	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	d.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	d.send(tea.KeyMsg{Type: tea.KeyEsc})
	if view := d.view(); !strings.Contains(view, "Options") || strings.Contains(view, "Execute mode will delete") {
		t.Fatalf("expected esc at the execute confirmation to return to the cleaner options:\n%s", view)
	}
}

func TestRouterTypesQIntoHostsField(t *testing.T) {
	d := newDriver(t)

	d.send(tea.KeyMsg{Type: tea.KeyDown})
	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	d.selectNetworkAction("Prompts for domain and IP")
	d.send(tea.KeyMsg{Type: tea.KeyEnter})
	d.send(key("q"))
	d.pumpFor(20 * time.Millisecond)

	if view := d.view(); d.quit || !strings.Contains(view, "Add a hosts entry") || !strings.Contains(view, "example.localq") {
		t.Fatalf("expected q to be typed into the domain field:\n%s", view)
	}
}

func TestRouterTypesQIntoMenuFilter(t *testing.T) {
	d := newDriver(t)

	d.send(key("/"))
	d.send(key("q"))
	d.pumpFor(20 * time.Millisecond)

	if view := d.view(); d.quit || !strings.Contains(view, "Filter: q") {
		t.Fatalf("expected q to be typed into the menu filter:\n%s", view)
	}
}

type driver struct {
	t      *testing.T
	router ui.Router
	msgs   chan tea.Msg
	quit   bool
}

func newDriver(t *testing.T, features ...ui.AppFeature) *driver {
	d := &driver{t: t, router: ui.NewRouter(features...), msgs: make(chan tea.Msg, 64)}
	d.send(tea.WindowSizeMsg{Width: 100, Height: 40})
	return d
}

func (d *driver) send(msg tea.Msg) {
	d.t.Helper()
	next, cmd := d.router.Update(msg)
	d.router = next.(ui.Router)
	d.start(cmd)
}

func (d *driver) start(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, next := range batch {
				d.start(next)
			}
			return
		}
		d.msgs <- msg
	}()
}

func (d *driver) handle(msg tea.Msg) {
	switch msg.(type) {
	case nil, spinner.TickMsg:
	case tea.QuitMsg:
		d.quit = true
	default:
		d.send(msg)
	}
}

func (d *driver) pumpUntil(done func() bool) {
	d.t.Helper()
	deadline := time.After(2 * time.Second)
	for !done() {
		select {
		case msg := <-d.msgs:
			d.handle(msg)
		case <-deadline:
			d.t.Fatalf("timed out waiting for the router; view:\n%s", d.view())
		}
	}
}

func (d *driver) pumpFor(duration time.Duration) {
	deadline := time.After(duration)
	for {
		select {
		case msg := <-d.msgs:
			d.handle(msg)
		case <-deadline:
			return
		}
	}
}

func (d *driver) selectNetworkAction(detail string) {
	d.t.Helper()
	for range 40 {
		if strings.Contains(d.view(), detail) {
			return
		}
		d.send(tea.KeyMsg{Type: tea.KeyDown})
	}
	d.t.Fatalf("network action with detail %q not found:\n%s", detail, d.view())
}

func (d *driver) view() string {
	return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(d.router.View(), "")
}

type blockingRun struct {
	started  chan struct{}
	canceled chan struct{}
	release  chan struct{}
}

func newBlockingRun() *blockingRun {
	return &blockingRun{started: make(chan struct{}), canceled: make(chan struct{}), release: make(chan struct{})}
}

func (r *blockingRun) block(ctx context.Context) {
	close(r.started)
	<-ctx.Done()
	close(r.canceled)
	<-r.release
}

func (r *blockingRun) cleaner(ctx context.Context, _ cleaner.Options) (cleaner.Report, error) {
	r.block(ctx)
	return cleaner.Report{Entries: []cleaner.Entry{{Level: cleaner.LevelInfo, Message: "fake run stopped"}}}, ctx.Err()
}

func (r *blockingRun) Output(ctx context.Context, _ string, _ ...string) ([]byte, error) {
	r.block(ctx)
	return nil, ctx.Err()
}

func (r *blockingRun) waitStarted(t *testing.T) {
	t.Helper()
	select {
	case <-r.started:
	case <-time.After(2 * time.Second):
		t.Fatal("run did not start")
	}
}

func (r *blockingRun) waitCanceled(t *testing.T) {
	t.Helper()
	select {
	case <-r.canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("run context was not canceled")
	}
}

type modelFeature struct {
	title string
	model func() tea.Model
}

func (f modelFeature) Title() string       { return f.title }
func (f modelFeature) Description() string { return f.title }
func (f modelFeature) Model() tea.Model    { return f.model() }

func cleanerFeature(run views.CleanerRunFunc) ui.AppFeature {
	return modelFeature{title: "Cleaner", model: func() tea.Model { return views.NewCleanerModelWithRunner(run) }}
}

func networkFeature(commands corenetwork.CommandRunner) ui.AppFeature {
	return modelFeature{title: "Network", model: func() tea.Model {
		return views.NewNetworkModelWithManager(corenetwork.NewNetworkManager(commands))
	}}
}
