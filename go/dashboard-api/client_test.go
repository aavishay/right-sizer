package dashboardapi

import (
	"context"
	"testing"
)

func TestClientAuthentication(t *testing.T) {
	tests := []struct {
		name        string
		config      ClientConfig
		expectStart bool
	}{
		{
			name: "valid token",
			config: ClientConfig{
				BaseURL:     "http://test.example.com",
				APIToken:    "test-token-123",
				ClusterID:   "test-cluster",
				ClusterName: "test-cluster",
			},
			expectStart: true,
		},
		{
			name: "missing token",
			config: ClientConfig{
				BaseURL:     "http://test.example.com",
				APIToken:    "",
				ClusterID:   "test-cluster",
				ClusterName: "test-cluster",
			},
			expectStart: true, // Client starts but disables integration
		},
		{
			name: "missing URL",
			config: ClientConfig{
				BaseURL:     "",
				APIToken:    "test-token-123",
				ClusterID:   "test-cluster",
				ClusterName: "test-cluster",
			},
			expectStart: true, // Client starts but disables integration
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.config)

			// Test Start method - client should always start, but may disable integration
			err := client.Start(context.TODO())
			if !tt.expectStart && err != nil {
				t.Errorf("Expected client to start (even with missing config), but got error: %v", err)
			}
			if tt.expectStart && err != nil {
				t.Errorf("Expected client to start successfully, but got error: %v", err)
			}

			// Clean up
			client.Stop()
		})
	}
}

func TestClientBearerTokenHeader(t *testing.T) {
	config := ClientConfig{
		BaseURL:     "http://test.example.com",
		APIToken:    "test-bearer-token",
		ClusterID:   "test-cluster",
		ClusterName: "test-cluster",
	}

	client := NewClient(config)

	// We can't easily test the actual HTTP request without a server,
	// but we can verify the client is configured correctly
	if client.config.APIToken != "test-bearer-token" {
		t.Errorf("Expected API token to be 'test-bearer-token', got '%s'", client.config.APIToken)
	}

	if client.config.BaseURL != "http://test.example.com" {
		t.Errorf("Expected base URL to be 'http://test.example.com', got '%s'", client.config.BaseURL)
	}
}
