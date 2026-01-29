package tui

import (
	"context"
	"fmt"
	"strings"

	"hooktm/internal/replay"
	"hooktm/internal/store"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func Run(ctx context.Context, s *store.Store, defaultTarget string) error {
	m := newModel(ctx, s, defaultTarget)
	p := tea.NewProgram(m, tea.WithContext(ctx))
	_, err := p.Run()
	if err != nil {
		return err
	}
	return nil
}

type model struct {
	ctx           context.Context
	store         *store.Store
	defaultTarget string

	rows   []store.WebhookSummary
	sel    int
	detail *store.Webhook

	search string
	err    error

	width  int
	height int
}

func newModel(ctx context.Context, s *store.Store, defaultTarget string) model {
	return model{ctx: ctx, store: s, defaultTarget: defaultTarget, sel: 0}
}

func (m model) Init() tea.Cmd {
	return m.loadListCmd("")
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case listLoadedMsg:
		m.rows = msg.rows
		if m.sel >= len(m.rows) {
			m.sel = max(0, len(m.rows)-1)
		}
		return m, m.loadDetailCmd()
	case detailLoadedMsg:
		m.detail = &msg.wh
		return m, nil
	case replayDoneMsg:
		// refresh detail after replay (not stored, but UX).
		m.err = msg.err
		return m, nil
	case errMsg:
		m.err = msg.err
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.sel > 0 {
				m.sel--
				return m, m.loadDetailCmd()
			}
		case "down", "j":
			if m.sel < len(m.rows)-1 {
				m.sel++
				return m, m.loadDetailCmd()
			}
		case "r":
			return m, m.replaySelectedCmd()
		case "/":
			// Clear search prompt; collect with simple input mode via m.search as buffer.
			m.search = ""
			return m, nil
		case "enter":
			// If search buffer non-empty, apply it.
			if strings.TrimSpace(m.search) != "" {
				return m, m.loadListCmd(strings.TrimSpace(m.search))
			}
		case "backspace":
			if len(m.search) > 0 {
				m.search = m.search[:len(m.search)-1]
			}
			return m, nil
		default:
			// naive search input: type any printable chars into search buffer.
			if len(msg.Runes) == 1 && msg.Runes[0] >= 32 && msg.Runes[0] != 127 {
				m.search += string(msg.Runes[0])
				return m, nil
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("HookTM")
	header := fmt.Sprintf("%s  target=%s", title, emptyTo(m.defaultTarget, "(none)"))
	if m.err != nil {
		header = header + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("error: "+m.err.Error())
	}
	if strings.TrimSpace(m.search) != "" {
		header = header + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render("search: "+m.search+" (Enter to apply)")
	} else {
		header = header + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("keys: j/k move, r replay, / search, q quit")
	}

	leftW := min(60, max(30, m.width/2))
	rightW := max(20, m.width-leftW-2)

	left := renderList(m.rows, m.sel, leftW, m.height-4)
	right := renderDetail(m.detail, rightW, m.height-4)

	return header + "\n\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

type listLoadedMsg struct{ rows []store.WebhookSummary }
type detailLoadedMsg struct{ wh store.Webhook }
type replayDoneMsg struct{ err error }
type errMsg struct{ err error }

func (m model) loadListCmd(search string) tea.Cmd {
	// Capture values to avoid race conditions.
	ctx := m.ctx
	st := m.store
	return func() tea.Msg {
		var (
			rows []store.WebhookSummary
			err  error
		)
		if strings.TrimSpace(search) != "" {
			rows, err = st.SearchSummaries(ctx, search, 200)
		} else {
			rows, err = st.ListSummaries(ctx, store.ListFilter{Limit: 200})
		}
		if err != nil {
			return errMsg{err: err}
		}
		return listLoadedMsg{rows: rows}
	}
}

func (m model) loadDetailCmd() tea.Cmd {
	// Capture values to avoid race conditions.
	ctx := m.ctx
	st := m.store
	rows := m.rows
	sel := m.sel
	return func() tea.Msg {
		if len(rows) == 0 {
			return detailLoadedMsg{wh: store.Webhook{}}
		}
		if sel >= len(rows) {
			sel = len(rows) - 1
		}
		id := rows[sel].ID
		wh, err := st.GetWebhook(ctx, id)
		if err != nil {
			return errMsg{err: err}
		}
		return detailLoadedMsg{wh: wh}
	}
}

func (m model) replaySelectedCmd() tea.Cmd {
	// Capture values to avoid race conditions.
	ctx := m.ctx
	st := m.store
	rows := m.rows
	sel := m.sel
	target := m.defaultTarget
	return func() tea.Msg {
		if len(rows) == 0 {
			return replayDoneMsg{err: nil}
		}
		if strings.TrimSpace(target) == "" {
			return replayDoneMsg{err: fmt.Errorf("no replay target configured")}
		}
		if sel >= len(rows) {
			sel = len(rows) - 1
		}
		id := rows[sel].ID
		engine := replay.NewEngine(st)
		_, err := engine.ReplayByID(ctx, id, target, "")
		return replayDoneMsg{err: err}
	}
}

func renderList(rows []store.WebhookSummary, sel, w, h int) string {
	var b strings.Builder
	for i, r := range rows {
		prefix := "  "
		if i == sel {
			prefix = "> "
		}
		status := "-"
		if r.StatusCode != nil {
			status = fmt.Sprintf("%d", *r.StatusCode)
		}
		prov := emptyTo(r.Provider, "unknown")
		line := fmt.Sprintf("%s%s %s %s [%s/%s] %dms", prefix, r.ID, r.Method, r.Path, prov, status, r.ResponseMS)
		line = truncate(line, w)
		b.WriteString(line)
		b.WriteString("\n")
		if b.Len() > w*h {
			break
		}
	}
	return lipgloss.NewStyle().Width(w).Height(h).Border(lipgloss.RoundedBorder()).Render(b.String())
}

func renderDetail(wh *store.Webhook, w, h int) string {
	if wh == nil || wh.ID == "" {
		return lipgloss.NewStyle().Width(w).Height(h).Border(lipgloss.RoundedBorder()).Render("No webhooks captured yet.")
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("ID: %s\n", wh.ID))
	b.WriteString(fmt.Sprintf("Provider: %s\n", emptyTo(wh.Provider, "unknown")))
	if strings.TrimSpace(wh.EventType) != "" {
		b.WriteString(fmt.Sprintf("Event: %s\n", wh.EventType))
	}
	if strings.TrimSpace(wh.Signature) != "" {
		b.WriteString(fmt.Sprintf("Signature: %s\n", truncate(wh.Signature, w-12)))
	}
	b.WriteString("\nHeaders:\n")
	// only show a few headers
	count := 0
	for k, vs := range wh.Headers {
		for _, v := range vs {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, truncate(v, w-6-len(k))))
			count++
			if count >= 10 {
				b.WriteString("  ...\n")
				goto body
			}
		}
	}
body:
	b.WriteString("\nBody:\n")
	bodyStr := string(wh.Body)
	if len(bodyStr) > 4000 {
		bodyStr = bodyStr[:4000] + "\n... (truncated)\n"
	}
	b.WriteString(bodyStr)

	return lipgloss.NewStyle().Width(w).Height(h).Border(lipgloss.RoundedBorder()).Render(b.String())
}

func truncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= maxW {
		return s
	}
	if maxW <= 1 {
		return string(rs[:maxW])
	}
	return string(rs[:maxW-1]) + "…"
}

func emptyTo(s, v string) string {
	if strings.TrimSpace(s) == "" {
		return v
	}
	return s
}
