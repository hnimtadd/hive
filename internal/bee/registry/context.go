package registry

import "context"

type contextKey string

const contextKeyRegistry contextKey = "registry"

func ContextWithRegistry(ctx context.Context, registry Registry) context.Context {
	return context.WithValue(ctx, contextKeyRegistry, registry)
}

func RegistryFromContext(ctx context.Context) (Registry, bool) {
	registryAny := ctx.Value(contextKeyRegistry)
	if registryAny == nil {
		return nil, false
	}
	registry, isRegistry := registryAny.(Registry)
	return registry, isRegistry
}
