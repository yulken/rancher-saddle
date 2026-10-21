package harvester

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHarvesterConfig_FromMap_Defaults(t *testing.T) {
	cfg := &HarvesterConfig{}
	cfg.FromMap(map[string]interface{}{})

	assert.Equal(t, defaultVersion, cfg.Version)
	assert.Equal(t, defaultClusterToken, cfg.ClusterToken)
	assert.Equal(t, defaultPassword, cfg.Password)
	assert.Equal(t, defaultVIP, cfg.VIP)
	assert.Equal(t, VIPModeStatic, cfg.VIPMode)
	assert.Equal(t, []string{defaultNTPServer}, cfg.NTPServers)
	assert.False(t, cfg.DeployRancher)
	assert.GreaterOrEqual(t, cfg.NodeCount, 1)
}

func TestHarvesterConfig_FromMap_Overrides(t *testing.T) {
	cfg := &HarvesterConfig{}
	cfg.FromMap(map[string]interface{}{
		"harvester_version": "v1.5.0",
		"cluster_token":     "mytoken",
		"password":          "s3cret",
		"vip":               "10.0.0.5",
		"vip_mode":          "static",
		"ntp_servers":       "a.ntp.org, b.ntp.org",
		"deploy_rancher":    true,
		"node_count":        3,
	})

	assert.Equal(t, "v1.5.0", cfg.Version)
	assert.Equal(t, "mytoken", cfg.ClusterToken)
	assert.Equal(t, "s3cret", cfg.Password)
	assert.Equal(t, "10.0.0.5", cfg.VIP)
	assert.Equal(t, "static", cfg.VIPMode)
	assert.Equal(t, []string{"a.ntp.org", "b.ntp.org"}, cfg.NTPServers)
	assert.True(t, cfg.DeployRancher)
	assert.Equal(t, 3, cfg.NodeCount)
}

func TestHarvesterConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     HarvesterConfig
		wantErr bool
	}{
		{"valid DHCP", HarvesterConfig{Version: "v1.4.1", VIP: "1.2.3.4", VIPMode: "DHCP", NodeCount: 1}, false},
		{"valid static", HarvesterConfig{Version: "v1.4.1", VIP: "1.2.3.4", VIPMode: "static", NodeCount: 3}, false},
		{"missing version", HarvesterConfig{VIP: "1.2.3.4", VIPMode: "DHCP", NodeCount: 1}, true},
		{"missing vip", HarvesterConfig{Version: "v1.4.1", VIPMode: "DHCP", NodeCount: 1}, true},
		{"bad vip mode", HarvesterConfig{Version: "v1.4.1", VIP: "1.2.3.4", VIPMode: "bogus", NodeCount: 1}, true},
		{"zero nodes", HarvesterConfig{Version: "v1.4.1", VIP: "1.2.3.4", VIPMode: "DHCP", NodeCount: 0}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHarvester_GetDefaultConfig(t *testing.T) {
	d := GetDefaultConfig()
	assert.Equal(t, defaultVersion, d["harvester_version"])
	assert.Equal(t, defaultVIP, d["vip"])
	assert.Contains(t, d, "cluster_token")
}

func TestHarvester_GetRequiredFields(t *testing.T) {
	fields := GetRequiredFields()
	assert.NotEmpty(t, fields)
	names := map[string]bool{}
	for _, f := range fields {
		names[f.Name] = true
	}
	assert.True(t, names["harvester_version"])
	assert.True(t, names["vip"])
	assert.True(t, names["vip_mode"])
}
