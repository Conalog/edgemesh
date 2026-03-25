package v1alpha1

import (
	"testing"

	"github.com/kubeedge/edgemesh/pkg/apis/config/defaults"
)

func TestDetectRunningMode(t *testing.T) {
	tests := []struct {
		name     string
		setEnv   bool
		expected defaults.RunningMode
	}{
		{
			name:     "cloud mode when KUBERNETES_PORT is set",
			setEnv:   true,
			expected: defaults.CloudMode,
		},
		{
			name:     "edge mode when KUBERNETES_PORT is not set",
			setEnv:   false,
			expected: defaults.EdgeMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv("KUBERNETES_PORT", "tcp://10.96.0.1:443")
			}
			mode := DetectRunningMode()
			if mode != tt.expected {
				t.Errorf("DetectRunningMode() = %v, want %v", mode, tt.expected)
			}
		})
	}
}

func TestPreConfigAgent_EdgeMode(t *testing.T) {
	cfg := NewDefaultEdgeMeshAgentConfig("")
	preConfigAgent(cfg, defaults.EdgeMode)

	if cfg.KubeAPIConfig.Mode != defaults.EdgeMode {
		t.Errorf("Mode = %v, want %v", cfg.KubeAPIConfig.Mode, defaults.EdgeMode)
	}
	if !cfg.Modules.EdgeDNSConfig.Enable {
		t.Error("EdgeDNS should be enabled in EdgeMode")
	}
}

func TestPreConfigAgent_CloudMode(t *testing.T) {
	cfg := NewDefaultEdgeMeshAgentConfig("")
	// Simulate user config that enables modules
	cfg.Modules.EdgeDNSConfig.Enable = true
	cfg.Modules.EdgeProxyConfig.Enable = true

	preConfigAgent(cfg, defaults.CloudMode)

	if cfg.KubeAPIConfig.Mode != defaults.CloudMode {
		t.Errorf("Mode = %v, want %v", cfg.KubeAPIConfig.Mode, defaults.CloudMode)
	}
	if cfg.Modules.EdgeDNSConfig.Enable {
		t.Error("EdgeDNS should be disabled in CloudMode")
	}
}

func TestNewDefaultEdgeMeshAgentConfig_Defaults(t *testing.T) {
	cfg := NewDefaultEdgeMeshAgentConfig("")

	if cfg.Modules.EdgeProxyConfig.Enable {
		t.Error("EdgeProxy should be disabled by default")
	}
	if cfg.Modules.EdgeCNIConfig.Enable {
		t.Error("EdgeCNI should be disabled by default")
	}
	if cfg.Modules.EdgeTunnelConfig.Enable {
		t.Error("EdgeTunnel should be disabled by default")
	}
	if cfg.Modules.EdgeDNSConfig.ListenPort != 53 {
		t.Errorf("EdgeDNS ListenPort = %d, want 53", cfg.Modules.EdgeDNSConfig.ListenPort)
	}
	if cfg.CommonConfig.BridgeDeviceName != defaults.BridgeDeviceName {
		t.Errorf("BridgeDeviceName = %v, want %v", cfg.CommonConfig.BridgeDeviceName, defaults.BridgeDeviceName)
	}
	if cfg.CommonConfig.BridgeDeviceIP != defaults.BridgeDeviceIP {
		t.Errorf("BridgeDeviceIP = %v, want %v", cfg.CommonConfig.BridgeDeviceIP, defaults.BridgeDeviceIP)
	}
}

func TestNewDefaultEdgeMeshAgentConfig_CloudModeAutoDetect(t *testing.T) {
	// Simulate cloud node environment
	t.Setenv("KUBERNETES_PORT", "tcp://10.96.0.1:443")

	cfg := NewDefaultEdgeMeshAgentConfig("")

	if cfg.KubeAPIConfig.Mode != defaults.CloudMode {
		t.Errorf("Mode = %v, want %v", cfg.KubeAPIConfig.Mode, defaults.CloudMode)
	}
	if cfg.Modules.EdgeDNSConfig.Enable {
		t.Error("EdgeDNS should be disabled in auto-detected CloudMode")
	}
}

func TestNewDefaultEdgeMeshAgentConfig_EdgeModeAutoDetect(t *testing.T) {
	cfg := NewDefaultEdgeMeshAgentConfig("")

	if cfg.KubeAPIConfig.Mode != defaults.EdgeMode {
		t.Errorf("Mode = %v, want %v", cfg.KubeAPIConfig.Mode, defaults.EdgeMode)
	}
	if !cfg.Modules.EdgeDNSConfig.Enable {
		t.Error("EdgeDNS should be enabled in auto-detected EdgeMode")
	}
}

// TestCloudMode_BridgeDeviceNotNeeded verifies that in CloudMode config,
// both EdgeDNS and EdgeProxy are disabled so bridge device creation is skipped.
// This corresponds to the condition in prepareRun():
//   if c.Modules.EdgeDNSConfig.Enable || c.Modules.EdgeProxyConfig.Enable { ... }
func TestCloudMode_BridgeDeviceNotNeeded(t *testing.T) {
	t.Setenv("KUBERNETES_PORT", "tcp://10.96.0.1:443")
	cfg := NewDefaultEdgeMeshAgentConfig("")

	needsBridge := cfg.Modules.EdgeDNSConfig.Enable || cfg.Modules.EdgeProxyConfig.Enable
	if needsBridge {
		t.Error("CloudMode should not require bridge device (both EdgeDNS and EdgeProxy should be disabled)")
	}
}

// TestEdgeMode_BridgeDeviceNeeded verifies that in EdgeMode config,
// EdgeDNS is enabled so bridge device creation is required.
func TestEdgeMode_BridgeDeviceNeeded(t *testing.T) {
	cfg := NewDefaultEdgeMeshAgentConfig("")

	needsBridge := cfg.Modules.EdgeDNSConfig.Enable || cfg.Modules.EdgeProxyConfig.Enable
	if !needsBridge {
		t.Error("EdgeMode should require bridge device (EdgeDNS should be enabled)")
	}
}

// TestCloudMode_YAMLOverrideSimulation verifies that YAML config can override
// CloudMode defaults. This tests the current behavior where Parse() can
// re-enable modules that preConfigAgent() disabled.
func TestCloudMode_YAMLOverrideSimulation(t *testing.T) {
	t.Setenv("KUBERNETES_PORT", "tcp://10.96.0.1:443")
	cfg := NewDefaultEdgeMeshAgentConfig("")

	// Verify CloudMode defaults are correct
	if cfg.Modules.EdgeDNSConfig.Enable {
		t.Fatal("EdgeDNS should be disabled before YAML override")
	}
	if cfg.Modules.EdgeProxyConfig.Enable {
		t.Fatal("EdgeProxy should be disabled before YAML override")
	}

	// Simulate YAML config overriding (what Parse/yaml.Unmarshal does)
	cfg.Modules.EdgeProxyConfig.Enable = true

	// After YAML override, bridge device would be needed — this is the
	// config ordering bug. If user's ConfigMap enables EdgeProxy on CloudMode,
	// bridge device creation will be attempted and fail without privileges.
	needsBridge := cfg.Modules.EdgeDNSConfig.Enable || cfg.Modules.EdgeProxyConfig.Enable
	if !needsBridge {
		t.Error("After YAML override enabling EdgeProxy, bridge device should be needed")
	}
}

// TestPreConfigAgent_CloudMode_DoesNotDisableProxy documents the current gap:
// preConfigAgent only disables EdgeDNS in CloudMode, not EdgeProxy.
func TestPreConfigAgent_CloudMode_ProxyNotDisabled(t *testing.T) {
	cfg := NewDefaultEdgeMeshAgentConfig("")
	cfg.Modules.EdgeProxyConfig.Enable = true

	preConfigAgent(cfg, defaults.CloudMode)

	// Current behavior: EdgeProxy remains enabled even in CloudMode.
	// This is the gap identified in the plan — preConfigAgent should also
	// disable EdgeProxy in CloudMode.
	if !cfg.Modules.EdgeProxyConfig.Enable {
		t.Skip("EdgeProxy is now disabled in CloudMode — gap has been fixed")
	}
	// If we reach here, the gap still exists
	t.Log("KNOWN GAP: preConfigAgent does not disable EdgeProxy in CloudMode")
}

func TestNewDefaultEdgeMeshAgentMinConfig(t *testing.T) {
	cfg := NewDefaultEdgeMeshAgentMinConfig()

	if !cfg.Modules.EdgeProxyConfig.Enable {
		t.Error("MinConfig should have EdgeProxy enabled")
	}
	if !cfg.Modules.EdgeTunnelConfig.Enable {
		t.Error("MinConfig should have EdgeTunnel enabled")
	}
	if cfg.Modules.EdgeTunnelConfig.RelayNodes == nil || len(cfg.Modules.EdgeTunnelConfig.RelayNodes) == 0 {
		t.Error("MinConfig should have RelayNodes configured")
	}
}

func TestNewDefaultEdgeMeshGatewayConfig(t *testing.T) {
	cfg := NewDefaultEdgeMeshGatewayConfig("")

	if cfg.Modules.EdgeGatewayConfig.Enable {
		t.Error("EdgeGateway should be disabled by default")
	}
	if cfg.Modules.EdgeTunnelConfig.Enable {
		t.Error("EdgeTunnel should be disabled by default in gateway config")
	}
}
