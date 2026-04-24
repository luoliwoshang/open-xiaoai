package plugins

import (
	"github.com/idootop/open-xiaoai/examples/go-instruction-server/internal/plugin"
	"github.com/idootop/open-xiaoai/examples/go-instruction-server/internal/plugins/canceltask"
	"github.com/idootop/open-xiaoai/examples/go-instruction-server/internal/plugins/complextask"
	"github.com/idootop/open-xiaoai/examples/go-instruction-server/internal/plugins/continuetask"
	"github.com/idootop/open-xiaoai/examples/go-instruction-server/internal/plugins/listtools"
	"github.com/idootop/open-xiaoai/examples/go-instruction-server/internal/plugins/querytaskprogress"
	"github.com/idootop/open-xiaoai/examples/go-instruction-server/internal/plugins/stock"
	"github.com/idootop/open-xiaoai/examples/go-instruction-server/internal/plugins/weather"
	"github.com/idootop/open-xiaoai/examples/go-instruction-server/internal/tasks"
)

func RegisterAll(registry *plugin.Registry, weatherService weather.Service, taskManager *tasks.Manager, complexTaskService *complextask.Service, resumeRegistry *continuetask.ResumeRegistry) error {
	if err := weather.Register(registry, weatherService); err != nil {
		return err
	}
	if err := stock.Register(registry); err != nil {
		return err
	}
	if err := complextask.Register(registry, complexTaskService); err != nil {
		return err
	}
	if err := continuetask.Register(registry, taskManager, resumeRegistry); err != nil {
		return err
	}
	if err := querytaskprogress.Register(registry, taskManager); err != nil {
		return err
	}
	if err := canceltask.Register(registry, taskManager); err != nil {
		return err
	}
	if err := listtools.Register(registry); err != nil {
		return err
	}
	return nil
}
