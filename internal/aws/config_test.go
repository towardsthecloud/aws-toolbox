package aws

import (
	"path/filepath"
	"testing"
)

func TestLoadAWSConfigWithRegionOverride(t *testing.T) {
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	cfg, err := LoadAWSConfig("", "us-east-1")
	if err != nil {
		t.Fatalf("LoadAWSConfig() error = %v", err)
	}
	if cfg.Region != "us-east-1" {
		t.Fatalf("expected region us-east-1, got %q", cfg.Region)
	}
}

func TestLoadAWSConfigWithProfile(t *testing.T) {
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	cfgDir := t.TempDir()

	configPath := filepath.Join(cfgDir, "config")
	credentialsPath := filepath.Join(cfgDir, "credentials")

	writeFile(t, configPath, "[profile test-profile]\nregion = us-west-2\n")
	writeFile(t, credentialsPath, "[test-profile]\naws_access_key_id = test\naws_secret_access_key = test\n")

	t.Setenv("AWS_CONFIG_FILE", configPath)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credentialsPath)

	cfg, err := LoadAWSConfig("test-profile", "")
	if err != nil {
		t.Fatalf("LoadAWSConfig() error = %v", err)
	}
	if cfg.Region != "us-west-2" {
		t.Fatalf("expected region us-west-2, got %q", cfg.Region)
	}
}

func TestLoadAWSConfigMissingProfileReturnsError(t *testing.T) {
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "missing-credentials"))

	_, err := LoadAWSConfig("does-not-exist", "")
	if err == nil {
		t.Fatal("expected error for missing profile")
	}
}
