package gotana

import (
	"github.com/op/go-logging"
	"os"
	"encoding/json"
	"strings"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
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