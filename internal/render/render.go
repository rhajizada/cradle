package render

import (
	"fmt"
	"io"

	"github.com/rhajizada/cradle/internal/service"
)

type Renderer struct {
	out io.Writer
}

func New(out io.Writer) *Renderer {
	return &Renderer{out: out}
}

func (r *Renderer) Aliases(infos []service.AliasInfo) {
	for _, info := range infos {
		fmt.Fprintf(r.out, "%-20s %s\n", info.Name, info.Description())
	}
}

func (r *Renderer) BuildStart(info service.AliasInfo) {
	switch info.Kind {
	case service.ImagePull:
		fmt.Fprintf(r.out, "pull: %s\n", info.Ref)
	case service.ImageBuild:
		fmt.Fprintf(r.out, "build: %s from %s\n", info.Tag, info.Cwd)
	}
}

func (r *Renderer) RunStart(id string) {
	fmt.Fprintf(r.out, "container: %s\n", id)
}
