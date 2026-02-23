package render

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"golang.org/x/term"

	"github.com/rhajizada/cradle/internal/termutil"

	"github.com/rhajizada/cradle/internal/service"
)

const (
	colImgStatus = 2
	colCtrStatus = 4

	zebraMod = 2

	statusColWidth = 12
)

// Renderer prints user-facing output for cradle commands.
type Renderer struct {
	log *slog.Logger
	out io.Writer
}

// New constructs a Renderer that logs to log and prints to out.
func New(log *slog.Logger, out io.Writer) *Renderer {
	return &Renderer{log: log, out: out}
}

// BuildStart emits a log line describing the start of a build/pull operation.
func (r *Renderer) BuildStart(info service.AliasInfo) {
	switch info.Kind {
	case service.ImagePull:
		r.log.Info("image pull", "ref", info.Ref)
	case service.ImageBuild:
		r.log.Info("image build", "tag", info.Tag, "context", info.Cwd)
	}
}

// ListStatuses renders a table of alias statuses.
func (r *Renderer) ListStatuses(items []service.AliasStatus) {
	if len(items) == 0 {
		_, _ = fmt.Fprintln(r.out, "No aliases found.")
		return
	}

	w := terminalWidth(r.out)
	_, _ = fmt.Fprint(r.out, renderAliasStatusTable(items, w))
}

// RunStart emits a log line indicating a container was started.
func (r *Renderer) RunStart(id string) {
	r.log.Info("container started", "id", id)
}

// RunStop emits a log line indicating a container was stopped.
func (r *Renderer) RunStop(id string) {
	r.log.Info("container stopped", "id", id)
}

// ImageStatusLabel returns an emoji label indicating whether an image exists locally.
func ImageStatusLabel(present bool) string {
	if present {
		return "✅"
	}
	return "❌"
}

// ImageStatusText returns a lowercase text label describing the image availability.
func ImageStatusText(present bool) string {
	if present {
		return "present"
	}
	return "missing"
}

// ContainerStatusLabel returns an emoji label describing the container state.
func ContainerStatusLabel(item service.AliasStatus) string {
	if !item.ContainerPresent {
		return "❌"
	}
	if item.ContainerStatus == "" {
		return "🤷"
	}
	switch item.ContainerStatus {
	case "running":
		return "▶️"
	case "exited":
		return "⛔️"
	case "created":
		return "✅"
	case "paused":
		return "⏸️"
	case "restarting":
		return "🔄"
	case "dead":
		return "💀"
	default:
		return "🤷"
	}
}

// ContainerStatusText returns a lowercase text label describing the container state.
func ContainerStatusText(item service.AliasStatus) string {
	if !item.ContainerPresent {
		return "missing"
	}
	if item.ContainerStatus == "" {
		return "unknown"
	}
	return strings.ToLower(item.ContainerStatus)
}

// terminalWidth returns the width of the terminal connected to w.
// It returns 0 if w is not a terminal or the width can't be determined.
func terminalWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 0
	}
	fd, ok := termutil.Int(f.Fd())
	if !ok {
		return 0
	}
	if !term.IsTerminal(fd) {
		return 0
	}
	width, _, err := term.GetSize(fd)
	if err != nil || width <= 0 {
		return 0
	}
	return width
}

func isStatusCol(col int) bool {
	return col == colImgStatus || col == colCtrStatus
}

// renderAliasStatusTable renders the alias status table.
// If width > 0, the table is constrained to that width and wrapping is enabled.
func renderAliasStatusTable(items []service.AliasStatus, width int) string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.Name,
			item.ImageRef,
			fmt.Sprintf("%s %s", ImageStatusLabel(item.ImagePresent), ImageStatusText(item.ImagePresent)),
			item.ContainerName,
			fmt.Sprintf("%s %s", ContainerStatusLabel(item), ContainerStatusText(item)),
		})
	}

	var (
		purple    = lipgloss.Color("99")
		gray      = lipgloss.Color("245")
		lightGray = lipgloss.Color("241")

		baseCell = lipgloss.NewStyle().Padding(0, 1)

		headerCell = baseCell.Foreground(purple).Bold(true).Align(lipgloss.Center)

		oddRowCell  = baseCell.Foreground(gray)
		evenRowCell = baseCell.Foreground(lightGray)
	)

	styleFor := func(row, col int) lipgloss.Style {
		if row == table.HeaderRow {
			s := headerCell
			if isStatusCol(col) {
				s = s.Width(statusColWidth).Align(lipgloss.Center)
			}
			return s
		}

		var s lipgloss.Style
		if row%zebraMod == 0 {
			s = evenRowCell
		} else {
			s = oddRowCell
		}

		if isStatusCol(col) {
			return s.Width(statusColWidth).Align(lipgloss.Left)
		}
		return s.Align(lipgloss.Left)
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(purple)).
		StyleFunc(styleFor).
		Headers("Alias", "Image", "Status", "Container", "Status").
		Rows(rows...)

	if width > 0 {
		t.Width(width).Wrap(true)
	}

	return t.String() + "\n"
}
