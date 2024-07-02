package workloads

import "testing"

func TestToManifestWorks(t *testing.T) {
	works, err := ToManifestWorks("test", "init")
	if err != nil {
		t.Error(err)
	}

	for _, work := range works {
		t.Errorf("%s: %d", work.Name, len(work.Spec.Workload.Manifests[0].Raw))
	}

}
