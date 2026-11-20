package env

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type TestConfig struct {
	Port        int           `env:"PORT" default:"8080"`
	Host        string        `env:"HOST" default:"0.0.0.0"`
	Debug       bool          `env:"DEBUG" default:"true"`
	Timeout     time.Duration `env:"TIMEOUT" default:"5s"`
	SecretKey   string        `env:"SECRET_KEY" required:"prod"`
	CallbackURL string        `env:"CALLBACK_URL" required:"prod" no_localhost:"prod"`
	Tags        []string      `env:"TAGS" default:"api,v1"`
	AllowedIPs  []int         `env:"ALLOWED_PORTS" default:"80,443"`
}

func TestEnv_LoadDefaults(t *testing.T) {
	os.Clearenv()

	var cfg TestConfig
	err := Load(&cfg, WithProd(false))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("expected Port=8080, got %d", cfg.Port)
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("expected Host=0.0.0.0, got %s", cfg.Host)
	}
	if !cfg.Debug {
		t.Errorf("expected Debug=true, got %v", cfg.Debug)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("expected Timeout=5s, got %v", cfg.Timeout)
	}
	if len(cfg.Tags) != 2 || cfg.Tags[0] != "api" || cfg.Tags[1] != "v1" {
		t.Errorf("expected Tags=[api, v1], got %v", cfg.Tags)
	}
	if len(cfg.AllowedIPs) != 2 || cfg.AllowedIPs[0] != 80 || cfg.AllowedIPs[1] != 443 {
		t.Errorf("expected AllowedIPs=[80, 443], got %v", cfg.AllowedIPs)
	}
}

func TestEnv_ProductionRequiredValidation(t *testing.T) {
	os.Clearenv()

	var cfg TestConfig
	// 在生产环境中，SecretKey 是 required:"prod"，CallbackURL 是 required:"prod"
	err := Load(&cfg, WithProd(true))
	if err == nil {
		t.Fatalf("expected error in production due to missing required keys, got nil")
	}

	valErrs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T: %v", err, err)
	}

	if len(valErrs) < 2 {
		t.Errorf("expected at least 2 validation errors, got %d: %v", len(valErrs), valErrs)
	}
}

func TestEnv_ProductionNoLocalhostValidation(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("SECRET_KEY", "super-secret")
	_ = os.Setenv("CALLBACK_URL", "http://localhost:3000/callback")

	var cfg TestConfig
	err := Load(&cfg, WithProd(true))
	if err == nil {
		t.Fatalf("expected error because CALLBACK_URL points to localhost in production")
	}

	_ = os.Setenv("CALLBACK_URL", "https://api.example.com/callback")
	err = Load(&cfg, WithProd(true))
	if err != nil {
		t.Fatalf("unexpected error with valid prod url: %v", err)
	}
}

func TestEnv_DotEnvFileLoading(t *testing.T) {
	os.Clearenv()

	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env.test")
	content := `
# 这是一个测试 .env 文件
PORT=9999
HOST="127.0.0.1" # 行尾注释
DEBUG=0
TIMEOUT=15s
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test env file: %v", err)
	}

	var cfg TestConfig
	err := Load(&cfg, WithEnvFiles(envPath), WithProd(false))
	if err != nil {
		t.Fatalf("Load with env file failed: %v", err)
	}

	if cfg.Port != 9999 {
		t.Errorf("expected Port=9999, got %d", cfg.Port)
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("expected Host=127.0.0.1, got %s", cfg.Host)
	}
	if cfg.Debug {
		t.Errorf("expected Debug=false, got %v", cfg.Debug)
	}
	if cfg.Timeout != 15*time.Second {
		t.Errorf("expected Timeout=15s, got %v", cfg.Timeout)
	}
}

func TestEnv_HelperFunctions(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("TEST_STR", "hello")
	_ = os.Setenv("TEST_INT", "123")
	_ = os.Setenv("TEST_BOOL", "yes")
	_ = os.Setenv("TEST_DUR", "2h")

	if GetString("TEST_STR", "def") != "hello" {
		t.Errorf("GetString failed")
	}
	if GetInt("TEST_INT", 0) != 123 {
		t.Errorf("GetInt failed")
	}
	if !GetBool("TEST_BOOL", false) {
		t.Errorf("GetBool failed")
	}
	if GetDuration("TEST_DUR", 0) != 2*time.Hour {
		t.Errorf("GetDuration failed")
	}
}
