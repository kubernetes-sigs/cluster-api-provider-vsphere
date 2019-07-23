/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kustomize

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"sigs.k8s.io/kustomize/k8sdeps/kunstruct"
	k8stransformer "sigs.k8s.io/kustomize/k8sdeps/transformer"
	"sigs.k8s.io/kustomize/k8sdeps/validator"
	"sigs.k8s.io/kustomize/pkg/fs"
	"sigs.k8s.io/kustomize/pkg/ifc"
	"sigs.k8s.io/kustomize/pkg/ifc/transformer"
	"sigs.k8s.io/kustomize/pkg/loader"
	"sigs.k8s.io/kustomize/pkg/plugins"
	"sigs.k8s.io/kustomize/pkg/resmap"
	"sigs.k8s.io/kustomize/pkg/resource"
	"sigs.k8s.io/kustomize/pkg/target"
	"sigs.k8s.io/kustomize/pkg/types"
	"sigs.k8s.io/kustomize/plugin/builtin"
)

// ReorderOutputType is the type of output ordering used by Kustomize.
type ReorderOutputType int

const (
	// UnspecifiedOutputOrder is illegal, either NoOutputOrder or
	// LegacyOutputOrder must be selected.
	UnspecifiedOutputOrder ReorderOutputType = iota

	// NoOutputOrder suppresses a final reordering.
	NoOutputOrder

	// LegacyOutputOrder is namespaces first, webhooks last, etc.
	LegacyOutputOrder
)

// BuildOptions are the options to pass to RunBuild in order to do a
// kustomization.
type BuildOptions struct {
	KustomizationPath       string
	Out                     io.Writer
	TemplateData            map[string]interface{}
	OutOrder                ReorderOutputType
	LoadRestrictorFunc      loader.LoadRestrictorFunc
	FileSystem              fs.FileSystem
	UnstructuredFactory     ifc.KunstructuredFactory
	PatchTransformerFactory transformer.Factory
	ResourceFactory         *resmap.Factory
	Validator               ifc.Validator
	PluginConfig            *types.PluginConfig
	PluginLoader            *plugins.Loader
}

// Run executes Kustomize against the provided root path and emits the results
// to the given io.Writer. If no writer is provided then os.Stdout is used.
func Run(kustomizationPath string, out io.Writer) error {
	return RunBuild(&BuildOptions{
		KustomizationPath: kustomizationPath,
		Out:               out,
	})
}

// RunBuild executes Kustomize with the provided build options.
// This is an advanced function, and Run should probably be used instead.
func RunBuild(opts *BuildOptions) error {

	// Ensure that none of the build options are set to their empty/nil values.
	opts.reconcileMissingOptions()

	// Check to see if the kustomization path is local, and if it is,
	// scan it for any *.template files. Those will need to be placed
	// into a hybrid FS that is backed by a real filesystem, but also
	// can return the interpolated templates.
	if err := opts.reconcileVirtualFileSystem(); err != nil {
		return errors.Wrap(err, "failed to reconcile virtual filesystem")
	}

	kustomizationLoader, err := loader.NewLoader(
		opts.LoadRestrictorFunc,
		opts.Validator,
		opts.KustomizationPath,
		opts.FileSystem)
	if err != nil {
		return errors.Wrap(err, "failed to create new loader")
	}
	defer kustomizationLoader.Cleanup()

	kustomizationTarget, err := target.NewKustTarget(
		kustomizationLoader,
		opts.ResourceFactory,
		opts.PatchTransformerFactory,
		opts.PluginLoader)
	if err != nil {
		return errors.Wrap(err, "failed to create new kustomization target")
	}

	customizedResourceMap, err := kustomizationTarget.MakeCustomizedResMap()
	if err != nil {
		return errors.Wrap(err, "failed to make customized resource map")
	}

	if opts.OutOrder == LegacyOutputOrder {
		// Done this way just to show how overall sorting can be performed by a
		// plugin. This particular plugin doesn't require configuration; just
		// make it and call transform.
		builtin.NewLegacyOrderTransformerPlugin().Transform(customizedResourceMap)
	}

	resourceMapAsYAML, err := customizedResourceMap.AsYaml()
	if err != nil {
		return errors.Wrap(err, "failed to marshal customized resource map to YAML")
	}

	if _, err := opts.Out.Write(resourceMapAsYAML); err != nil {
		return errors.Wrap(err, "failed to write marshaled resources")
	}

	return nil
}

func (o *BuildOptions) reconcileMissingOptions() {
	if o.Out == nil {
		o.Out = os.Stdout
	}
	if o.TemplateData == nil {
		o.TemplateData = map[string]interface{}{}
	}
	if o.OutOrder == UnspecifiedOutputOrder {
		o.OutOrder = LegacyOutputOrder
	}
	if o.LoadRestrictorFunc == nil {
		o.LoadRestrictorFunc = loader.RestrictionRootOnly
	}
	if o.FileSystem == nil {
		o.FileSystem = fs.MakeRealFS()
	}
	if o.UnstructuredFactory == nil {
		o.UnstructuredFactory = kunstruct.NewKunstructuredFactoryImpl()
	}
	if o.PatchTransformerFactory == nil {
		o.PatchTransformerFactory = k8stransformer.NewFactoryImpl()
	}
	if o.ResourceFactory == nil {
		o.ResourceFactory = resmap.NewFactory(resource.NewFactory(o.UnstructuredFactory))
	}
	if o.Validator == nil {
		o.Validator = validator.NewKustValidator()
	}
	if o.PluginConfig == nil {
		o.PluginConfig = plugins.DefaultPluginConfig()
	}
	if o.PluginLoader == nil {
		o.PluginLoader = plugins.NewLoader(o.PluginConfig, o.ResourceFactory)
	}
}

func (o *BuildOptions) reconcileVirtualFileSystem() error {
	vfs := &virtualFileSystem{
		FileSystem: o.FileSystem,
		fake:       fs.MakeFakeFS(),
	}
	o.FileSystem = vfs
	if err := filepath.Walk(o.KustomizationPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".template") {
			return nil
		}

		// Parse the template file.
		tpl, err := template.ParseFiles(path)
		if err != nil {
			return err
		}

		// Execute the template to build the interpolated data. It's better to
		// write this to a buffer instead of directly to fakeFile below since
		// fake files don't support buffered writes.
		interpolatedData := &bytes.Buffer{}
		if err := tpl.Execute(interpolatedData, o.TemplateData); err != nil {
			return err
		}

		// Create a new fake file to hold the interpolated data. Please note
		// the file is created without the ".template" extension.
		fakeFilePath := strings.TrimSuffix(path, filepath.Ext(path))
		fakeFile, err := vfs.fake.Create(fakeFilePath)
		if err != nil {
			return err
		}
		defer fakeFile.Close()

		// Write the interpolated data to the fake file.
		if _, err := fakeFile.Write(interpolatedData.Bytes()); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "failed to reocncile virtual file system")
	}
	return nil
}

type virtualFileSystem struct {
	fs.FileSystem // real
	fake          fs.FileSystem
}

func (vfs *virtualFileSystem) CleanedAbs(path string) (fs.ConfirmedDir, string, error) {
	// The fake filesystem's CleanedAbs call cannot fail, so first check to
	// see if the path exists.
	if vfs.fake.Exists(path) {
		return vfs.fake.CleanedAbs(path)
	}
	return vfs.FileSystem.CleanedAbs(path)
}

func (vfs *virtualFileSystem) Open(name string) (fs.File, error) {
	if file, err := vfs.fake.Open(name); err == nil {
		return file, nil
	}
	return vfs.FileSystem.Open(name)
}

func (vfs *virtualFileSystem) Exists(name string) bool {
	if ok := vfs.fake.Exists(name); ok {
		return true
	}
	return vfs.FileSystem.Exists(name)
}

func (vfs *virtualFileSystem) Glob(pattern string) ([]string, error) {
	if files, err := vfs.fake.Glob(pattern); err == nil {
		return files, nil
	}
	return vfs.FileSystem.Glob(pattern)
}

func (vfs *virtualFileSystem) ReadFile(name string) ([]byte, error) {
	if data, err := vfs.fake.ReadFile(name); err == nil {
		return data, nil
	}
	return vfs.FileSystem.ReadFile(name)
}
