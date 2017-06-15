package gotana

import (
	"encoding/json"
	"github.com/op/go-logging"
	yaml "gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"runtime"
	"strings"
)

func Logger() *logging.Logger {
	logger := logging.MustGetLogger("gotana")
	backend := logging.NewLogBackend(os.Stdout, "", 0)
	format := logging.MustStringFormatter(
		`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.7s} %{color:reset} %{message}`,
	)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)
	return logger
}

func ProcessFile(config interface{}, file string) error {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	switch {
	case strings.HasSuffix(file, ".yaml") || strings.HasSuffix(file, ".yml"):
		return yaml.Unmarshal(data, config)
	case strings.HasSuffix(file, ".json"):
		return json.Unmarshal(data, config)
	default:
		return nil
	}
}

func SilentRecover(name string) {
	if r := recover(); r != nil {
		Logger().Warningf("Recovered %s", name)
	}
}

func GetMapKeys(m map[string]interface{}) []string {
	keys := make([]string, len(m))

	i := 0
	for key := range m {
		keys[i] = key
		i++
	}

	return keys
}

func DisplayBytes(bytes []byte) {
	Logger().Info(string(bytes))
}

func DisplayResponseBody(r io.Reader) {
	bodyBytes, _ := ioutil.ReadAll(r)
	DisplayBytes(bodyBytes)
}

func StripString(s string) string {
	result := s
	stripCharacters := []string{" ", "\n", "\t"}

	for _, char := range stripCharacters {
		result = strings.Replace(result, char, "", -1)
	}

	return result
}

func DescribeFunc(f interface{}) string {
	v := reflect.ValueOf(f)

	if v.Kind() == reflect.Func {
		if rf := runtime.FuncForPC(v.Pointer()); rf != nil {
			return rf.Name()
		}
	}
	return v.String()
}

func DescribeStruct(v interface{}) string {
	valueOf := reflect.ValueOf(v)

	if valueOf.Type().Kind() == reflect.Ptr {
		return reflect.Indirect(valueOf).Type().Name()
	} else {
		return valueOf.Type().Name()
	}
}

func ContainsOneOf(s string, targets []string) bool {
	for _, el := range targets {
		if strings.Contains(s, el) {
			return true
		}
	}
	return false
}
