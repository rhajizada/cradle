package render

import (
	"log/slog"

	"github.com/rhajizada/cradle/internal/service"
)

type Renderer struct {
	log *slog.Logger
}

func New(log *slog.Logger) *Renderer {
	return &Renderer{log: log}
}

func (r *Renderer) Aliases(infos []service.AliasInfo) {
	for _, info := range infos {
		r.log.Info("alias", "name", info.Name, "kind", info.Kind, "detail", info.Description())
	}
}

func (r *Renderer) BuildStart(info service.AliasInfo) {
	switch info.Kind {
	case service.ImagePull:
		r.log.Info("image pull", "ref", info.Ref)
	case service.ImageBuild:
		r.log.Info("image build", "tag", info.Tag, "context", info.Cwd)
	}
}

func (r *Renderer) RunStart(id string) {
	r.log.Info("container started", "id", id)
}

func (r *Renderer) RunStop(id string) {
	r.log.Info("container stopped", "id", id)
}
