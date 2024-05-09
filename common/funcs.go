package common

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/desc"
)

func GetMethods(source grpcurl.DescriptorSource, configs map[string]*SvcConfig) ([]*desc.MethodDescriptor, error) {
	allServices, err := source.ListServices()
	if err != nil {
		return nil, err
	}

	var descs []*desc.MethodDescriptor
	for _, svc := range allServices {
		if svc == "grpc.reflection.v1alpha.ServerReflection" || svc == "grpc.reflection.v1.ServerReflection" {
			continue
		}
		d, err := source.FindSymbol(svc)
		if err != nil {
			return nil, err
		}
		sd, ok := d.(*desc.ServiceDescriptor)
		if !ok {
			return nil, fmt.Errorf("%s should be a service descriptor but instead is a %T", d.GetFullyQualifiedName(), d)
		}
		cfg := configs[svc]
		if cfg == nil && len(configs) != 0 {
			// not configured to expose this service
			continue
		}
		delete(configs, svc)
		for _, md := range sd.GetMethods() {
			if cfg == nil {
				descs = append(descs, md)
				continue
			}
			_, found := cfg.IncludeMethods[md.GetName()]
			delete(cfg.IncludeMethods, md.GetName())
			if found && cfg.IncludeService {
				Warn("Service %s already configured, so -method %s is unnecessary", svc, md.GetName())
			}
			if found || cfg.IncludeService {
				descs = append(descs, md)
			}
		}
		if cfg != nil && len(cfg.IncludeMethods) > 0 {
			// configured methods not found
			methodNames := make([]string, 0, len(cfg.IncludeMethods))
			for m := range cfg.IncludeMethods {
				methodNames = append(methodNames, fmt.Sprintf("%s/%s", svc, m))
			}
			sort.Strings(methodNames)
			return nil, fmt.Errorf("configured methods not found: %s", strings.Join(methodNames, ", "))
		}
	}

	if len(configs) > 0 {
		// configured services not found
		svcNames := make([]string, 0, len(configs))
		for s := range configs {
			svcNames = append(svcNames, s)
		}
		sort.Strings(svcNames)
		return nil, fmt.Errorf("configured services not found: %s", strings.Join(svcNames, ", "))
	}

	return descs, nil
}

type SvcConfig struct {
	IncludeService bool
	IncludeMethods map[string]struct{}
}

func Warn(msg string, args ...interface{}) {
	msg = fmt.Sprintf("Warning: %s\n", msg)
	fmt.Fprintf(os.Stderr, msg, args...)
}
