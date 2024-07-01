package util

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"sigs.k8s.io/yaml"
)

const clusterNamePrefix = "maestro-cluster"

type Func func() error

func ClusterName(index int) string {
	return fmt.Sprintf("%s-%d", clusterNamePrefix, index)
}

func UsedTime(start time.Time, unit time.Duration) time.Duration {
	used := time.Since(start)
	return used / unit
}

func Eventually(fn Func, timeout time.Duration, interval time.Duration) error {
	after := time.After(timeout)

	tick := time.NewTicker(interval)
	defer tick.Stop()

	var err error
	for {
		select {
		case <-after:
			return fmt.Errorf("timeout with error %v", err)
		case <-tick.C:
			err = fn()

			if err == nil {
				return nil
			}
		}
	}
}

func Render(name string, data []byte, config interface{}) ([]byte, error) {
	tmpl, err := template.New(name).Parse(string(data))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return nil, err
	}

	return yaml.YAMLToJSON(buf.Bytes())
}
