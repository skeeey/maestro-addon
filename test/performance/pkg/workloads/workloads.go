package workloads

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	workv1 "open-cluster-management.io/api/work/v1"
)

var guestbook = []string{
	"manifests/guestbook/namespace.yaml",
	"manifests/guestbook/frontend-deployment.yaml",
	"manifests/guestbook/frontend-service.yaml",
	"manifests/guestbook/redis-master-deployment.yaml",
	"manifests/guestbook/redis-master-service.yaml",
	"manifests/guestbook/redis-slave-deployment.yaml",
	"manifests/guestbook/redis-slave-service.yaml",
}

var (
	genericScheme = runtime.NewScheme()
	genericCodecs = serializer.NewCodecFactory(genericScheme)
	genericCodec  = genericCodecs.UniversalDeserializer()
)

func init() {
	utilruntime.Must(appsv1.AddToScheme(genericScheme))
	utilruntime.Must(corev1.AddToScheme(genericScheme))
}

//go:embed manifests
var ManifestFiles embed.FS

func CopyWork(batch int, work *workv1.ManifestWork) *workv1.ManifestWork {
	newWork := &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name: WorkName(batch, work.Name),
			Labels: map[string]string{
				"maestro.performance.test": "acm",
			},
			Annotations: work.Annotations,
		},
		Spec: workv1.ManifestWorkSpec{
			Workload:        work.Spec.Workload,
			DeleteOption:    work.Spec.DeleteOption,
			ManifestConfigs: work.Spec.ManifestConfigs,
		},
	}

	for k, v := range work.Labels {
		newWork.Labels[k] = v
	}

	return newWork
}

func WorkName(batch int, name string) string {
	return fmt.Sprintf("%s-%d", name, batch)
}

func ToGuestBookWorks(clusterName string, total int) ([]*workv1.ManifestWork, error) {
	workloads, err := guestBookWorkLoad()
	if err != nil {
		return nil, err
	}

	works := []*workv1.ManifestWork{}
	for i := 0; i < total; i++ {
		works = append(works, &workv1.ManifestWork{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-gb-%d", clusterName, i),
				Labels: map[string]string{
					"maestro.performance.test": "guest-book",
				},
			},
			Spec: workv1.ManifestWorkSpec{
				Workload: workv1.ManifestsTemplate{Manifests: workloads},
				ManifestConfigs: []workv1.ManifestConfigOption{
					{
						ResourceIdentifier: workv1.ResourceIdentifier{
							Resource: "namespaces",
							Name:     "playback-ns",
						},
						FeedbackRules: []workv1.FeedbackRule{
							{
								Type: workv1.JSONPathsType,
								JsonPaths: []workv1.JsonPath{
									{
										Name: "status",
										Path: ".status",
									},
								},
							},
						},
					},
					{
						ResourceIdentifier: workv1.ResourceIdentifier{
							Group:     "apps",
							Resource:  "deployments",
							Namespace: "playback-ns",
							Name:      "frontend",
						},
						FeedbackRules: []workv1.FeedbackRule{
							{
								Type: workv1.JSONPathsType,
								JsonPaths: []workv1.JsonPath{
									{
										Name: "status",
										Path: ".status",
									},
								},
							},
						},
					},
					{
						ResourceIdentifier: workv1.ResourceIdentifier{
							Resource:  "services",
							Namespace: "playback-ns",
							Name:      "frontend",
						},
						FeedbackRules: []workv1.FeedbackRule{
							{
								Type: workv1.JSONPathsType,
								JsonPaths: []workv1.JsonPath{
									{
										Name: "status",
										Path: ".status",
									},
								},
							},
						},
					},
					{
						ResourceIdentifier: workv1.ResourceIdentifier{
							Group:     "apps",
							Resource:  "deployments",
							Namespace: "playback-ns",
							Name:      "redis-master",
						},
						FeedbackRules: []workv1.FeedbackRule{
							{
								Type: workv1.JSONPathsType,
								JsonPaths: []workv1.JsonPath{
									{
										Name: "status",
										Path: ".status",
									},
								},
							},
						},
					},
					{
						ResourceIdentifier: workv1.ResourceIdentifier{
							Resource:  "services",
							Namespace: "playback-ns",
							Name:      "redis-master",
						},
						FeedbackRules: []workv1.FeedbackRule{
							{
								Type: workv1.JSONPathsType,
								JsonPaths: []workv1.JsonPath{
									{
										Name: "status",
										Path: ".status",
									},
								},
							},
						},
					},
					{
						ResourceIdentifier: workv1.ResourceIdentifier{
							Group:     "apps",
							Resource:  "deployments",
							Namespace: "playback-ns",
							Name:      "redis-slave",
						},
						FeedbackRules: []workv1.FeedbackRule{
							{
								Type: workv1.JSONPathsType,
								JsonPaths: []workv1.JsonPath{
									{
										Name: "status",
										Path: ".status",
									},
								},
							},
						},
					},
					{
						ResourceIdentifier: workv1.ResourceIdentifier{
							Resource:  "services",
							Namespace: "playback-ns",
							Name:      "redis-slave",
						},
						FeedbackRules: []workv1.FeedbackRule{
							{
								Type: workv1.JSONPathsType,
								JsonPaths: []workv1.JsonPath{
									{
										Name: "status",
										Path: ".status",
									},
								},
							},
						},
					},
				},
			},
		})
	}
	return works, nil
}

func guestBookWorkLoad() ([]workv1.Manifest, error) {
	manifests := []workv1.Manifest{}
	for _, file := range guestbook {
		data, err := ManifestFiles.ReadFile(file)
		if err != nil {
			return nil, err
		}

		raw, err := createAssetFromTemplate(file, data, nil)
		if err != nil {
			return nil, err
		}

		obj, _, err := genericCodec.Decode(raw, nil, nil)
		if err != nil {
			return nil, err
		}

		manifests = append(manifests, toManifest(obj))
	}
	return manifests, nil
}

func createAssetFromTemplate(name string, tb []byte, config interface{}) ([]byte, error) {
	tmpl, err := template.New(name).Parse(string(tb))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func toManifest(object runtime.Object) workv1.Manifest {
	manifest := workv1.Manifest{}
	manifest.Object = object
	return manifest
}
