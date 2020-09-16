package logging

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	logrusLogstash "github.com/bshuster-repo/logrus-logstash-hook"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogConfig_Apply_getWriter(t *testing.T) {
	c := LogConfig{Writer: StdErr}
	assert.Equal(t, c.getWriter(), os.Stderr)

	c = LogConfig{Writer: StdOut}
	assert.Equal(t, c.getWriter(), os.Stdout)

	c = LogConfig{Writer: Discard}
	assert.Equal(t, c.getWriter(), ioutil.Discard)

	// fallback to stderr for unknown writer
	c = LogConfig{Writer: LogWriter("unknown")}
	assert.Equal(t, c.getWriter(), os.Stderr)
}

func TestLogConfig_Apply_getFormatter(t *testing.T) {
	c := LogConfig{Format: Text}
	assert.IsType(t, &log.TextFormatter{}, c.getFormatter())

	c = LogConfig{Format: JSON}
	assert.IsType(t, &log.JSONFormatter{}, c.getFormatter())

	// fallback to text for unknown format
	c = LogConfig{Format: LogFormat("unknown")}
	assert.IsType(t, &log.TextFormatter{}, c.getFormatter())

	c = LogConfig{Format: Logstash}
	formatter := c.getFormatter()
	require.IsType(t, &logrusLogstash.LogstashFormatter{}, formatter)

	logstashFormatter, _ := formatter.(*logrusLogstash.LogstashFormatter)
	assert.Equal(t, "", logstashFormatter.Type)
	assert.Equal(t, "", logstashFormatter.TimestampFormat)

	testType := "TestType"

	c = LogConfig{Format: Logstash, FormatSettings: map[string]string{"type": testType, "ts": "RFC3339"}}
	formatter = c.getFormatter()
	require.IsType(t, &logrusLogstash.LogstashFormatter{}, formatter)

	logstashFormatter, _ = formatter.(*logrusLogstash.LogstashFormatter)
	assert.Equal(t, testType, logstashFormatter.Type)
	assert.Equal(t, time.RFC3339, logstashFormatter.TimestampFormat)

	c = LogConfig{Format: Logstash, FormatSettings: map[string]string{"type": testType, "ts": "RFC3339Nano"}}
	formatter = c.getFormatter()
	require.IsType(t, &logrusLogstash.LogstashFormatter{}, formatter)

	logstashFormatter, _ = formatter.(*logrusLogstash.LogstashFormatter)
	assert.Equal(t, testType, logstashFormatter.Type)
	assert.Equal(t, time.RFC3339Nano, logstashFormatter.TimestampFormat)
}

func TestLogConfig_Apply(t *testing.T) {
	c := LogConfig{Level: "warning"}

	err := c.Apply()
	assert.NoError(t, err)

	assert.Equal(t, log.WarnLevel, log.GetLevel())

	c.Level = "unknown"
	err = c.Apply()
	assert.Error(t, err)
}

func TestLogConfig_Apply_initHooks(t *testing.T) {
	c := LogConfig{Hooks: LogHooks{LogHook{Format: "unknown", Settings: map[string]string{"host": "host", "port": "1234"}}}}
	err := c.initHooks()
	assert.Equal(t, ErrUnknownLogHookFormat, err)

	c = LogConfig{Hooks: LogHooks{LogHook{Format: "does-not-matter"}}}
	err = c.initHooks()
	assert.Equal(t, ErrMissingLogHookSetting, err)

	c = LogConfig{Hooks: LogHooks{LogHook{Format: HookLogstash, Settings: map[string]string{"host": "host", "port": "1234"}}}}
	err = c.initHooks()
	assert.Equal(t, ErrMissingLogHookSetting, err)

	c = LogConfig{Hooks: LogHooks{LogHook{Format: HookSyslog, Settings: map[string]string{"host": "host", "port": "1234"}}}}
	err = c.initHooks()
	assert.Equal(t, ErrMissingLogHookSetting, err)

	c = LogConfig{Hooks: LogHooks{LogHook{Format: HookSyslog, Settings: map[string]string{"network": "udp", "host": "host", "port": "1234"}}}}
	err = c.initHooks()
	assert.Equal(t, ErrFailedToConfigureLogHook, err)

	c = LogConfig{Hooks: LogHooks{LogHook{Format: HookGraylog, Settings: map[string]string{"host": "host", "port": "1234", "async": "???"}}}}
	err = c.initHooks()
	assert.Equal(t, ErrFailedToConfigureLogHook, err)
}

func TestLogHooks_UnmarshalText(t *testing.T) {
	setGlobalConfigEnv(t)
	require.NoError(t, os.Setenv("LOG_HOOKS", `[{broken:json"]}`))

	v := viper.New()
	InitDefaults(v, "")

	_, err := LoadConfigFromEnv(v)
	assert.Error(t, err)
}

func TestLoad(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	//assert.Contains(t, wd, "github.com/hellofresh/logging-go") // ?? Why does this matter?

	// .../github.com/hellofresh/logging-go/assets/config.yml
	configPath := filepath.Join(wd, "assets", "config.yml")
	_, err = os.Stat(configPath)
	assert.NoError(t, err)

	v := viper.New()
	InitDefaults(v, "")

	logConfig, err := Load(v, configPath)
	require.NoError(t, err)

	assertConfig(t, logConfig)
}

func TestLoad_fallbackToEnv(t *testing.T) {
	setGlobalConfigEnv(t)

	v := viper.New()
	InitDefaults(v, "")

	logConfig, err := Load(v, "")
	require.NoError(t, err)

	assertConfig(t, logConfig)
}

func TestLoadConfigFromEnv(t *testing.T) {
	setGlobalConfigEnv(t)

	v := viper.New()
	InitDefaults(v, "")

	logConfig, err := LoadConfigFromEnv(v)
	require.NoError(t, err)

	assertConfig(t, logConfig)
}

func TestLoadConfigFromViper(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)

	configPath := filepath.Join(wd, "assets", "config_from_viper.yml")
	_, err = os.Stat(configPath)
	assert.NoError(t, err)

	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	assert.NoError(t, err)

	logConfig, err := LoadConfigFromViper(v, "log")
	require.NoError(t, err)

	assertConfig(t, logConfig)
}

func setGlobalConfigEnv(t *testing.T) {
	t.Helper()

	require.NoError(t, os.Setenv("LOG_LEVEL", "info"))
	require.NoError(t, os.Setenv("LOG_FORMAT", "logstash"))
	require.NoError(t, os.Setenv("LOG_FORMAT_SETTINGS", "type:MyService,ts:RFC3339Nano"))
	require.NoError(t, os.Setenv("LOG_WRITER", "stderr"))
	require.NoError(t, os.Setenv("LOG_HOOKS", `[{"format":"logstash", "settings":{"type":"MyService","ts":"RFC3339Nano", "network": "udp","host":"logstash.mycompany.io","port": "8911"}},{"format":"syslog","settings":{"network": "udp", "host":"localhost", "port": "514", "tag": "MyService", "facility": "LOG_LOCAL0", "severity": "LOG_INFO"}},{"format":"graylog","settings":{"host":"graylog.mycompany.io","port":"9000"}},{"format":"stackdriver", "settings":{"service":"myservice","version":"v1"}}]`))
}

func assertConfig(t *testing.T, logConfig LogConfig) {
	assert.Equal(t, "info", logConfig.Level)
	assert.Equal(t, Logstash, logConfig.Format)
	assert.Equal(t, map[string]string{"type": "MyService", "ts": "RFC3339Nano"}, logConfig.FormatSettings)
	assert.Equal(t, StdErr, logConfig.Writer)

	assert.Equal(t, 4, len(logConfig.Hooks))

	// Check logstash
	assert.Equal(t, "logstash", logConfig.Hooks[0].Format)
	assert.Equal(t, map[string]string{"type": "MyService", "ts": "RFC3339Nano", "network": "udp", "host": "logstash.mycompany.io", "port": "8911"}, logConfig.Hooks[0].Settings)

	// Check syslog
	assert.Equal(t, "syslog", logConfig.Hooks[1].Format)
	assert.Equal(t, map[string]string{"network": "udp", "host": "localhost", "port": "514", "tag": "MyService", "facility": "LOG_LOCAL0", "severity": "LOG_INFO"}, logConfig.Hooks[1].Settings)

	// Check graylog
	assert.Equal(t, "graylog", logConfig.Hooks[2].Format)
	assert.Equal(t, map[string]string{"host": "graylog.mycompany.io", "port": "9000"}, logConfig.Hooks[2].Settings)

	// Check stackdriver
	assert.Equal(t, "stackdriver", logConfig.Hooks[3].Format)
	assert.Equal(t, map[string]string{"service": "myservice", "version": "v1"}, logConfig.Hooks[3].Settings)

}

func TestInitDefaults(t *testing.T) {
	vEmptyPrefix := viper.New()
	InitDefaults(vEmptyPrefix, "")

	assert.Equal(t, defaultLevel, vEmptyPrefix.Get("level"))
	assert.Equal(t, defaultFormat, vEmptyPrefix.Get("format"))
	assert.Equal(t, defaultWriter, vEmptyPrefix.Get("writer"))

	vWithPrefix := viper.New()
	InitDefaults(vWithPrefix, "prefix")

	assert.Equal(t, defaultLevel, vWithPrefix.Get("prefix.level"))
	assert.Equal(t, defaultFormat, vWithPrefix.Get("prefix.format"))
	assert.Equal(t, defaultWriter, vWithPrefix.Get("prefix.writer"))

	vWithPrefixDot := viper.New()
	InitDefaults(vWithPrefixDot, "prefix.")

	assert.Equal(t, defaultLevel, vWithPrefixDot.Get("prefix.level"))
	assert.Equal(t, defaultFormat, vWithPrefixDot.Get("prefix.format"))
	assert.Equal(t, defaultWriter, vWithPrefixDot.Get("prefix.writer"))
}
