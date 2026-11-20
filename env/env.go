package env

import (
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Option 定义配置加载选项
type Option func(*loadOptions)

type loadOptions struct {
	envFiles       []string
	isProd         *bool
	overrideSystem bool
}

// WithEnvFiles 指定要加载的 .env 文件列表（按顺序加载，后加载的不会覆盖已存在的键）
func WithEnvFiles(files ...string) Option {
	return func(o *loadOptions) {
		o.envFiles = files
	}
}

// WithProd 指定是否处于生产环境。如果不指定，默认根据 APP_ENV/ENV/PRODUCTION 环境变量自动判断
func WithProd(isProd bool) Option {
	return func(o *loadOptions) {
		o.isProd = &isProd
	}
}

// WithOverrideSystem 指定 .env 中的配置是否覆盖系统已有的环境变量（默认 false，即系统环境变量优先）
func WithOverrideSystem(override bool) Option {
	return func(o *loadOptions) {
		o.overrideSystem = override
	}
}

// ValidationError 环境变量校验错误集合
type ValidationError struct {
	Field   string
	EnvKey  string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("env: field %s (key: %s) validation failed: %s", e.Field, e.EnvKey, e.Message)
}

// ValidationErrors 多个校验错误的聚合
type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("env: %d validation error(s) occurred:\n", len(v)))
	for _, err := range v {
		sb.WriteString("  - ")
		sb.WriteString(err.Error())
		sb.WriteString("\n")
	}
	return sb.String()
}

// Load 将环境变量（以及默认的 .env 文件）安全加载并映射进 target 结构体指针中
func Load(target any, opts ...Option) error {
	options := &loadOptions{
		envFiles: []string{".env"},
	}
	for _, opt := range opts {
		opt(options)
	}

	// 1. 尝试读取 .env 文件并注入环境变量
	for _, file := range options.envFiles {
		_ = loadDotEnvFile(file, options.overrideSystem)
	}

	// 2. 判定生产环境状态
	isProd := IsProd()
	if options.isProd != nil {
		isProd = *options.isProd
	}

	// 3. 反射解析结构体
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("env: target must be a non-nil pointer to a struct, got %T", target)
	}

	val := v.Elem()
	var errs ValidationErrors

	if err := parseStruct(val, isProd, &errs); err != nil {
		return err
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func parseStruct(val reflect.Value, isProd bool, errs *ValidationErrors) error {
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		fieldVal := val.Field(i)
		fieldType := typ.Field(i)

		// 忽略非导出字段
		if !fieldVal.CanSet() {
			continue
		}

		// 递归处理嵌套结构体
		if fieldVal.Kind() == reflect.Struct && fieldType.Anonymous {
			if err := parseStruct(fieldVal, isProd, errs); err != nil {
				return err
			}
			continue
		}

		envTag := fieldType.Tag.Get("env")
		defaultVal := fieldType.Tag.Get("default")
		requiredTag := strings.ToLower(fieldType.Tag.Get("required"))
		noLocalhostTag := strings.ToLower(fieldType.Tag.Get("no_localhost"))

		if envTag == "" {
			continue
		}

		// 支持逗号分隔的多 Key 查找兜底：env:"PORT,APP_PORT"
		keys := strings.Split(envTag, ",")
		var rawVal string
		var matchedKey string
		var found bool

		for _, k := range keys {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			if v, exists := os.LookupEnv(k); exists && strings.TrimSpace(v) != "" {
				rawVal = strings.TrimSpace(v)
				matchedKey = k
				found = true
				break
			}
		}

		if !found {
			matchedKey = strings.TrimSpace(keys[0])
			rawVal = defaultVal
		}

		// 检查必填项规则
		isRequired := false
		if requiredTag == "true" || requiredTag == "1" {
			isRequired = true
		} else if (requiredTag == "prod" || requiredTag == "production") && isProd {
			isRequired = true
		}

		if isRequired && strings.TrimSpace(rawVal) == "" {
			*errs = append(*errs, ValidationError{
				Field:   fieldType.Name,
				EnvKey:  matchedKey,
				Message: "is required but missing or empty",
			})
			continue
		}

		// 检查 no_localhost 规则
		forbidLocal := false
		if noLocalhostTag == "true" || noLocalhostTag == "1" {
			forbidLocal = true
		} else if (noLocalhostTag == "prod" || noLocalhostTag == "production") && isProd {
			forbidLocal = true
		}

		if forbidLocal && rawVal != "" && isLocalhost(rawVal) {
			*errs = append(*errs, ValidationError{
				Field:   fieldType.Name,
				EnvKey:  matchedKey,
				Message: fmt.Sprintf("must not point to localhost in production (got: %s)", rawVal),
			})
			continue
		}

		// 如果没有值且非必填，直接跳过赋值
		if rawVal == "" {
			continue
		}

		// 类型转换与赋值
		if err := setFieldValue(fieldVal, rawVal); err != nil {
			*errs = append(*errs, ValidationError{
				Field:   fieldType.Name,
				EnvKey:  matchedKey,
				Message: fmt.Sprintf("type conversion error: %v", err),
			})
		}
	}

	return nil
}

func setFieldValue(field reflect.Value, raw string) error {
	// 特殊类型 time.Duration
	if field.Type() == reflect.TypeOf(time.Duration(0)) {
		d, err := time.ParseDuration(raw)
		if err != nil {
			// 如果仅为纯数字，默认按毫秒或秒解析？标准库推荐直接报错或规范时间格式（如 10s）
			return fmt.Errorf("invalid duration format: %w", err)
		}
		field.Set(reflect.ValueOf(d))
		return nil
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(val)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(val)

	case reflect.Bool:
		val, err := parseBool(raw)
		if err != nil {
			return err
		}
		field.SetBool(val)

	case reflect.Float32, reflect.Float64:
		val, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return err
		}
		field.SetFloat(val)

	case reflect.Slice:
		elemKind := field.Type().Elem().Kind()
		parts := strings.Split(raw, ",")
		var cleanParts []string
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				cleanParts = append(cleanParts, s)
			}
		}

		if elemKind == reflect.String {
			field.Set(reflect.ValueOf(cleanParts))
			return nil
		}

		if elemKind == reflect.Int {
			intSlice := make([]int, 0, len(cleanParts))
			for _, p := range cleanParts {
				iv, err := strconv.Atoi(p)
				if err != nil {
					return fmt.Errorf("invalid int in slice: %w", err)
				}
				intSlice = append(intSlice, iv)
			}
			field.Set(reflect.ValueOf(intSlice))
			return nil
		}

		return fmt.Errorf("unsupported slice element type: %s", elemKind)

	default:
		return fmt.Errorf("unsupported field kind: %s", field.Kind())
	}

	return nil
}

func parseBool(str string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(str)) {
	case "1", "t", "true", "yes", "y", "on":
		return true, nil
	case "0", "f", "false", "no", "n", "off", "":
		return false, nil
	}
	return false, fmt.Errorf("invalid boolean value: %q", str)
}

func isLocalhost(raw string) bool {
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "localhost") || strings.Contains(lower, "127.0.0.1") || strings.Contains(lower, "::1") {
		return true
	}
	if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
		h := strings.ToLower(u.Hostname())
		return h == "localhost" || h == "127.0.0.1" || h == "::1"
	}
	return false
}

// IsProd 自动通过环境变量判断当前是否为生产环境
func IsProd() bool {
	keys := []string{"APP_ENV", "ENV", "PRODUCTION", "MALL_PRODUCTION", "MALL_PROD"}
	for _, k := range keys {
		val := strings.ToLower(strings.TrimSpace(os.Getenv(k)))
		if val == "prod" || val == "production" || val == "true" || val == "1" {
			return true
		}
	}
	return false
}

// ------------------------------------------------------------
// 极简纯标准库 .env 解析器 (零外部依赖)
// ------------------------------------------------------------

func loadDotEnvFile(filename string, override bool) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		// 优先处理非引号内的行尾注释
		if strings.HasPrefix(val, "\"") {
			// 双引号包裹的值：找到闭合双引号
			if endQuote := strings.Index(val[1:], "\""); endQuote != -1 {
				val = val[1 : endQuote+1]
			}
		} else if strings.HasPrefix(val, "'") {
			// 单引号包裹的值：找到闭合单引号
			if endQuote := strings.Index(val[1:], "'"); endQuote != -1 {
				val = val[1 : endQuote+1]
			}
		} else {
			// 未包裹引号的值：去掉 # 注释
			if commentIdx := strings.Index(val, "#"); commentIdx != -1 {
				val = strings.TrimSpace(val[:commentIdx])
			}
		}

		if _, exists := os.LookupEnv(key); !exists || override {
			_ = os.Setenv(key, val)
		}
	}

	return scanner.Err()
}

// ------------------------------------------------------------
// 快捷便利函数
// ------------------------------------------------------------

// GetString 读取字符串环境变量，若为空返回 defaultVal
func GetString(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}

// GetInt 读取整型环境变量，若为空或解析失败返回 defaultVal
func GetInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		if iv, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			return iv
		}
	}
	return defaultVal
}

// GetBool 读取布尔环境变量，若为空或解析失败返回 defaultVal
func GetBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if bv, err := parseBool(val); err == nil {
			return bv
		}
	}
	return defaultVal
}

// GetDuration 读取 Duration 环境变量，若为空或解析失败返回 defaultVal
func GetDuration(key string, defaultVal time.Duration) time.Duration {
	if val, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(strings.TrimSpace(val)); err == nil {
			return d
		}
	}
	return defaultVal
}
