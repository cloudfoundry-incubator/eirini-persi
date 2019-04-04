package operator

import (
	"context"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	credsgen "code.cloudfoundry.org/cf-operator/pkg/credsgen/in_memory_generator"

	"github.com/SUSE/eirini-extensions/pkg/controllers"
	"github.com/SUSE/eirini-extensions/pkg/util/config"
	"github.com/SUSE/eirini-extensions/pkg/util/ctxlog"
)

// NewManager adds schemes, controllers and starts the manager
func NewManager(ctx context.Context, config *config.Config, cfg *rest.Config, options manager.Options) (mgr manager.Manager, err error) {
	mgr, err = manager.New(cfg, options)
	if err != nil {
		return
	}

	log := ctxlog.ExtractLogger(ctx)

	log.Info("Registering Components.")
	config.Namespace = options.Namespace

	// Setup Scheme for all resources
	if err = controllers.AddToScheme(mgr.GetScheme()); err != nil {
		return
	}

	// Setup Hooks for all resources
	if err = controllers.AddHooks(ctx, config, mgr, credsgen.NewInMemoryGenerator(log)); err != nil {
		return
	}

	// Setup all Controllers
	err = controllers.AddToManager(ctx, config, mgr)
	return
}
