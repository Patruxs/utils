package ui

import (
	"os"
	"os/user"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"utils/internal/ui/common"
)

const (
	helpLabelWidth   = 10
	helpKeyGap       = "   "
	helpIndent       = " "
	helpMaxWidth     = 76
	helpPaddingRight = 2
)

type helpSection struct {
	label string
	keys  []key.Binding
}

type appInfo struct {
	version string
	goos    string
	goarch  string
	account string
	built   string
}

func currentAppInfo(version string) appInfo {
	version = strings.TrimSpace(version)
	if version == "" {
		version = "dev"
	}
	return appInfo{
		version: version,
		goos:    runtime.GOOS,
		goarch:  runtime.GOARCH,
		account: currentAccount(),
		built:   buildDate(),
	}
}

func currentAccount() string {
	name := ""
	if current, err := user.Current(); err == nil {
		name = current.Username
		if index := strings.LastIndexAny(name, `\/`); index >= 0 {
			name = name[index+1:]
		}
	}
	host, err := os.Hostname()
	if err != nil || host == "" || name == "" {
		return name
	}
	return name + "@" + strings.SplitN(host, ".", 2)[0]
}

func buildDate() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.time" && len(setting.Value) >= len("2006-01-02") {
			return setting.Value[:len("2006-01-02")]
		}
	}
	return ""
}

func (i appInfo) platform() string {
	return i.goos + "/" + i.goarch
}

func (i appInfo) headerChoices() []string {
	return []string{
		joinInfo(i.version, i.platform(), i.account),
		joinInfo(i.version, i.platform()),
		joinInfo(i.version, i.goos),
		i.version,
	}
}

func (i appInfo) aboutLine() string {
	built := ""
	if i.built != "" {
		built = "built " + i.built
	}
	return joinInfo(i.version, i.platform(), i.account, built)
}

func joinInfo(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " · ")
}

func binding(keys, help string) key.Binding {
	return key.NewBinding(key.WithKeys(keys), key.WithHelp(keys, help))
}

var (
	globalHelpKeys = []key.Binding{
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab / 1 / 2", "switch tool")),
		binding("?", "help"),
		binding("q", "quit"),
	}
	cleanerHelpKeys = []key.Binding{
		binding("space", "toggle"),
		binding("enter", "delete…"),
		key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("←→", "scope / preview")),
		key.NewBinding(key.WithKeys("pgup", "pgdown"), key.WithHelp("pgup pgdn", "scroll preview")),
		binding("d", "save dry-run log"),
	}
	networkHelpKeys = []key.Binding{
		binding("enter", "run"),
		binding("space", "batch"),
		binding("r", "refresh status"),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field in a form")),
	}
	runningHelpKeys = []key.Binding{
		binding("esc", "cancel"),
		binding("ctrl+c", "cancel and quit"),
	}
	confirmHelpKeys = []key.Binding{
		key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("←→", "choose")),
		binding("y", "confirm"),
		key.NewBinding(key.WithKeys("esc", "n"), key.WithHelp("esc / n", "cancel")),
	}
)

func (m Router) helpView(maxWidth, maxHeight int) string {
	inner := common.Panel{Width: common.MinInt(maxWidth, helpMaxWidth)}.InnerWidth() - len(helpIndent)

	sections := []helpSection{{"GLOBAL", globalHelpKeys}}
	for _, tab := range m.tabs {
		if feature, ok := tab.feature.(helpKeysFeature); ok {
			sections = append(sections, helpSection{strings.ToUpper(tab.feature.Title()), feature.HelpKeys()})
		}
	}
	sections = append(sections, helpSection{"CONFIRM", confirmHelpKeys}, helpSection{"RUNNING", runningHelpKeys})

	var table []string
	for _, section := range sections {
		table = append(table, helpRows(section.label, section.keys, inner)...)
	}

	about := []string{m.info.aboutLine()}
	logo := strings.TrimRight(appLogo, "\r\n")
	chromeRows := 2 + 1 + len(about) + 1 + len(table) + 2
	if lipgloss.Width(logo) <= inner && chromeRows+lipgloss.Height(logo) <= maxHeight {
		about = append(strings.Split(common.Accent.Render(logo), "\n"), about...)
	} else {
		about = append([]string{common.Wordmark()}, about...)
	}

	rows := append([]string{""}, about...)
	rows = append(rows, "")
	rows = append(rows, table...)
	rows = append(rows, "", common.Hints([]key.Binding{shellKeys.Back}))
	natural := 0
	for i, row := range rows {
		rows[i] = helpIndent + row
		natural = common.MaxInt(natural, lipgloss.Width(rows[i]))
	}
	panel := common.Panel{Variant: common.PanelFocused, Width: common.MinInt(maxWidth, natural+helpPaddingRight+4)}
	return panel.Render(strings.Join(rows, "\n"))
}

func helpRows(label string, bindings []key.Binding, width int) []string {
	indent := strings.Repeat(" ", helpLabelWidth)
	room := common.MaxInt(1, width-helpLabelWidth)
	var rows []string
	line := ""
	for _, b := range bindings {
		part := common.Hints([]key.Binding{b})
		if line != "" && lipgloss.Width(line)+len(helpKeyGap)+lipgloss.Width(part) > room {
			rows = append(rows, line)
			line = ""
		}
		if line != "" {
			line += helpKeyGap
		}
		line += part
	}
	if line != "" {
		rows = append(rows, line)
	}
	for i := range rows {
		prefix := indent
		if i == 0 {
			prefix = common.PadRight(common.Muted.Bold(true).Render(label), helpLabelWidth)
		}
		rows[i] = prefix + rows[i]
	}
	return rows
}
