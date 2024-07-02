package workloads

import (
	"embed"

	"github.com/stolostron/maestro-addon/test/performance/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"

	workv1 "open-cluster-management.io/api/work/v1"
)

//go:embed manifests
var ManifestFiles embed.FS

var workloadFiles = []string{
	"manifests/configmap-big.yaml",
	"manifests/configmap.yaml",
	"manifests/deployment.yaml",
	"manifests/role.yaml",
	"manifests/rolebinding.yaml",
	"manifests/secret.yaml",
	"manifests/serviceaccount.yaml",
}

func ToManifestWorks(clusterName string, phase string) ([]*workv1.ManifestWork, error) {
	manifests, err := readFiles(clusterName)
	if err != nil {
		return nil, err
	}

	works := []*workv1.ManifestWork{}

	for _, manifest := range manifests {
		works = append(works, &workv1.ManifestWork{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rand.String(20),
				Namespace: clusterName,
				Labels: map[string]string{
					"phase.maestro.performance.test": phase,
					"type.maestro.performance.test":  "single",
				},
			},
			Spec: workv1.ManifestWorkSpec{
				Workload: workv1.ManifestsTemplate{
					Manifests: []workv1.Manifest{{
						RawExtension: runtime.RawExtension{Raw: manifest},
					}},
				},
			},
		})
	}

	works = append(works, newBundleWork(clusterName, phase, manifests))
	works = append(works, newBundleWork(clusterName, phase, manifests[1:]))
	works = append(works, newBundleWork(clusterName, phase, manifests[1:]))
	return works, nil
}

func readFiles(clusterName string) ([][]byte, error) {
	files := [][]byte{}
	for _, file := range workloadFiles {
		data, err := ManifestFiles.ReadFile(file)
		if err != nil {
			return nil, err
		}

		raw, err := util.Render(
			file,
			data,
			&struct {
				Name        string
				ClusterName string
			}{
				Name:        rand.String(20),
				ClusterName: clusterName,
			},
		)
		if err != nil {
			return nil, err
		}

		files = append(files, raw)
	}

	return files, nil
}

func newBundleWork(clusterName string, phase string, manifests [][]byte) *workv1.ManifestWork {
	work := &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.String(20),
			Namespace: clusterName,
			Labels: map[string]string{
				"phase.maestro.performance.test": phase,
				"type.maestro.performance.test":  "multiple",
			},
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{},
			},
		},
	}

	for _, manifest := range manifests {
		work.Spec.Workload.Manifests = append(work.Spec.Workload.Manifests, workv1.Manifest{
			RawExtension: runtime.RawExtension{Raw: manifest},
		})
	}
	return work
}
