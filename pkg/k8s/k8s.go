package k8s

import (
	"context"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"k8s.io/cli-runtime/pkg/resource"
	"github.com/markbates/pkger"
)

type EnsureServerOptions struct {
	KustomizationPath string
	Namespace
}

// EnsureServer ensures that a name_manager server is present in a
// Kubernetes cluster, with the latest version.  If it is not
// present, it is created.  If it is present but the version is old,
// it is upgraded.  If the cluster version is newer than the local version,
// an error is thrown.
//
// The call is idempotent, and multiple calls to EnsureServer can be
// made against the same cluster, at the same time.
func EnsureServer(ctx context.Context, client *rest.RESTClient, kustomizationPath string, namespace string, force bool) error {

}

func apply(ctx context.Context, client *rest.RESTClient, kustomizationPath, namespace string, force bool) error {
	fSys := filesys.MakeFsOnDisk()
	k := krusty.MakeKustomizer(fSys, krusty.MakeDefaultOptions())
	m, err := k.Run(kustomizationPath)
	if err != nil {
		return err
	}

	for _, res := range m.Resources() {
		data, err := res.AsYAML()
		if err != nil {
			return err
		}

		_, err = client.Patch(types.ApplyPatchType).
			Namespace(namespace).
			Resource(res.GetKind()).
			Name(res.GetName()).
			VersionedParams(
				&metav1.PatchOptions{
					Force: &force,
				},
				metav1.ParameterCodec).
			Body(data).
			Do().
			Get()
		if err != nil {
			return err
		}
	}

	return nil
}
