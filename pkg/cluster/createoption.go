/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cluster

import (
	"fmt"
	"time"

	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	internalcreate "sigs.k8s.io/kind/pkg/cluster/internal/create"
	internalconfig "sigs.k8s.io/kind/pkg/internal/apis/config"
	internalencoding "sigs.k8s.io/kind/pkg/internal/apis/config/encoding"
)

// CreateOption is a Provider.Create option
type CreateOption interface {
	apply(*internalcreate.ClusterOptions) error
}

type createOptionAdapter func(*internalcreate.ClusterOptions) error

func (c createOptionAdapter) apply(o *internalcreate.ClusterOptions) error {
	return c(o)
}

// CreateWithConfigFile configures the config file path to use
func CreateWithConfigFile(path string) CreateOption {
	return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
		var err error
		o.Config, err = internalencoding.Load(path)
		return err
	})
}

// CreateWithRawConfig configures the config to use from raw (yaml) bytes
func CreateWithRawConfig(raw []byte) CreateOption {
	return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
		var err error
		o.Config, err = internalencoding.Parse(raw)
		return err
	})
}

// CreateWithV1Alpha4Config configures the cluster with a v1alpha4 config
func CreateWithV1Alpha4Config(config *v1alpha4.Cluster) CreateOption {
	return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
		o.Config = internalencoding.V1Alpha4ToInternal(config)
		return nil
	})
}

// CreateWithNodeImage overrides the image on all nodes in config
// as an easy way to change the Kubernetes version
func CreateWithNodeImage(nodeImage string) CreateOption {
	return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
		o.NodeImage = nodeImage
		return nil
	})
}

// CreateWithRetain disables deletion of nodes and any other cleanup
// that would normally occur after a failure to create
// This is mainly used for debugging purposes
func CreateWithRetain(retain bool) CreateOption {
	return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
		o.Retain = retain
		return nil
	})
}

// CreateWithWaitForReady configures a maximum wait time for the control plane
// node(s) to be ready. By default no waiting is performed
func CreateWithWaitForReady(waitTime time.Duration) CreateOption {
	return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
		o.WaitForReady = waitTime
		return nil
	})
}

// CreateWithKubeconfigPath sets the explicit --kubeconfig path
func CreateWithKubeconfigPath(explicitPath string) CreateOption {
	return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
		o.KubeconfigPath = explicitPath
		return nil
	})
}

// CreateWithStopBeforeSettingUpKubernetes enables skipping setting up
// kubernetes (kubeadm init etc.) after creating node containers
// This generally shouldn't be used and is only lightly supported, but allows
// provisioning node containers for experimentation
func CreateWithStopBeforeSettingUpKubernetes(stopBeforeSettingUpKubernetes bool) CreateOption {
	return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
		o.StopBeforeSettingUpKubernetes = stopBeforeSettingUpKubernetes
		return nil
	})
}

// CreateWithDisplayUsage enables displaying usage if displayUsage is true
func CreateWithDisplayUsage(displayUsage bool) CreateOption {
	return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
		o.DisplayUsage = displayUsage
		return nil
	})
}

// CreateWithDisplaySalutation enables display a salutation at the end of create
// cluster if displaySalutation is true
func CreateWithDisplaySalutation(displaySalutation bool) CreateOption {
	return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
		o.DisplaySalutation = displaySalutation
		return nil
	})
}

// CreateWithAirGapped configures the cluster to fail fast if required images are missing locally
// instead of pulling them from the internet.
func CreateWithAirGapped(airGapped bool) CreateOption {
	return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
		o.AirGapped = airGapped
		return nil
	})
}

// CreateWithAddonProfile applies a named addon preset to the cluster configuration.
// An empty profile is a no-op; existing configuration is unchanged.
// Valid profiles: minimal, full, gateway, ci.
func CreateWithAddonProfile(profile string) CreateOption {
	return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
		if profile == "" {
			return nil
		}
		if o.Config == nil {
			cfg, err := internalencoding.Load("")
			if err != nil {
				return err
			}
			o.Config = cfg
		}
		switch profile {
		case "minimal":
			o.Config.Addons = internalconfig.Addons{} // zero value = all false
		case "full":
			o.Config.Addons = internalconfig.Addons{
				MetalLB: true, EnvoyGateway: true, MetricsServer: true,
				CoreDNSTuning: true, Dashboard: true, LocalRegistry: true, CertManager: true,
			}
		case "gateway":
			o.Config.Addons = internalconfig.Addons{MetalLB: true, EnvoyGateway: true}
		case "ci":
			o.Config.Addons = internalconfig.Addons{MetricsServer: true, CertManager: true}
		default:
			return fmt.Errorf("unknown profile %q: valid values are minimal, full, gateway, ci", profile)
		}
		return nil
	})
}
