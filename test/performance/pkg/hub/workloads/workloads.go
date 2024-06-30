package workloads

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/yaml"

	workv1 "open-cluster-management.io/api/work/v1"
)

//go:embed manifests
var ManifestFiles embed.FS

type RenderConfig struct {
	WorkName    string
	ClusterName string
}

var workloadFiles = []string{
	"manifests/configmap.yaml",
	"manifests/deployment.yaml",
	"manifests/role.yaml",
	"manifests/rolebinding.yaml",
	"manifests/secret.yaml",
	"manifests/serviceaccount.yaml",
}

func ToManifestWorks(clusterName string, workType string) ([]*workv1.ManifestWork, error) {
	works := []*workv1.ManifestWork{}

	for _, file := range workloadFiles {
		raw, err := render(file, &RenderConfig{
			WorkName:    fmt.Sprintf("%s-%s", clusterName, workType),
			ClusterName: clusterName,
		})
		if err != nil {
			return nil, err
		}

		works = append(works, &workv1.ManifestWork{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", clusterName, rand.String(10)),
				Namespace: clusterName,
				Labels: map[string]string{
					"maestro.performance.test": workType,
				},
			},
			Spec: workv1.ManifestWorkSpec{
				Workload: workv1.ManifestsTemplate{
					Manifests: []workv1.Manifest{{
						RawExtension: runtime.RawExtension{Raw: raw},
					}},
				},
			},
		})
	}

	return works, nil
}

func render(file string, config interface{}) ([]byte, error) {
	data, err := ManifestFiles.ReadFile(file)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(file).Parse(string(data))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return nil, err
	}

	return yaml.YAMLToJSON(buf.Bytes())
}
