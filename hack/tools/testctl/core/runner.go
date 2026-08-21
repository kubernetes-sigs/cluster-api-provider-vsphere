/*
Copyright The Kubernetes Authors.

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

package core

import (
	"context"
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"

	pkgerrors "github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// rootPath define the path of the root field.
var rootPath = field.NewPath("$")

// TestObjects is a map of key/objects to be passed to a plugin, e.g. "Cluster":..., "MachineDeployment":...
// Note: We are using this approach instead of adding info to the context because it is easier to debug.
type TestObjects map[string]any

// RunConfig defines the run config to be used when invoking plugins.
// Note: For the sake of simplicity there is a unique RunConfig for all the plugins; as a consequence some field in this struct
// might not be relevant for a specific plugin. Some field in this struct are managed by the runner itself, e.g. concurrency.
type RunConfig struct {
	// Limit defines the max number of TestObjects that will be considered for this plugin.
	// If Limit is not set, all candidate TestObjects will be considered.
	// Note: Selectors plugins might want to use this flag to avoid selecting more TestObjects than necessary
	Limit *int32 `json:"limit,omitempty"`

	// concurrency defines the number of TestObjects to be tested in parallel.
	// If not set, it defaults to 1.
	Concurrency *int32 `json:"concurrency,omitempty"`

	// failFast defines the behavior in case a test action fails.
	// If not set, it defaults to false.
	// Note that if concurrency is greater than 1, the system will try to complete other in-flight tests before exiting.
	// TODO: think about using this for failures that could happen to a plugin in the middle of a list of plugins (run field). Currently it is always fail fast
	FailFast *bool `json:"failFast,omitempty"`

	// dryRun instructs plugins to run in dry run mode (no change applied).
	// If not set, it defaults to false or to the value set in the top level RunConfig.
	// Note: SelectorPlugins are expected to return TestObjects no matter of the value of this field.
	DryRun *bool `json:"dryRun,omitempty"`

	// skipWait instructs plugins to not wait for an action to complete before returning.
	// If not set, it defaults to false.
	SkipWait *bool `json:"skipWait,omitempty"`

	// debug instructs runner to prompt a message before running a plugin.
	// If not set, it defaults to false or to the value set in the top level RunConfig.
	Debug *bool `json:"debug,omitempty"`

	// debugOnError instructs runner to prompt a message after a plugin fails.
	// If not set, it defaults to false or to the value set in the top level RunConfig.
	DebugOnError *bool `json:"debugOnError,omitempty"`

	// timeout for the plugin call.
	// If not set, the runner will wait for the plugin call to complete (no timeout).
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Intervals for the plugin call, if any.
	// Note: Intervals in the top level RunConfig are always available; in case the same
	// interval is defined both in the top level RunConfig and in a lower level RunConfig, the latter is used.
	Intervals map[string][]metav1.Duration `json:"intervals,omitempty"`
}

// Run the tests defined in the config using the given plugins.
func Run(ctx context.Context, c client.Client, config []byte, plugins map[string]any) error {
	for k, plugin := range plugins {
		isSelector := false
		if _, ok := plugin.(SelectorPlugin); ok {
			isSelector = true
		}
		isExecutor := false
		if _, ok := plugin.(ExecutorPlugin); ok {
			isExecutor = true
		}
		if !isSelector && !isExecutor {
			return pkgerrors.Errorf("plugin %s must implement one of SelectorPlugin, ExecutorPlugin", k)
		}
		if isSelector && isExecutor {
			return pkgerrors.Errorf("plugin %s cannot implement both SelectorPlugin and ExecutorPlugin", k)
		}
	}

	// Adds core plugins.
	// Note: core plugins are always added and cannot be overridden.
	plugins[importerPluginKey] = &importerPlugin{}

	r := newRunner(plugins)

	// Start the test by using an internal plugin, the rootSelectorPlugin that makes it possible
	// to go through the top level run config once, thus allowing invoking the plugins referenced by the config.
	rootPlugin := &rootSelectorPlugin{}

	// Read the rawConfig and store configs for different tests in r.configs.
	// During this phase also validate the configs (we don't want the test to fail in the middle of a run for a configuration error).
	if err := r.parseConfig(ctx, rootPath, rootPlugin, nil, config); err != nil {
		return pkgerrors.Wrap(err, "failed to validate config")
	}

	// Run the plugin.
	if err := r.runOne(ctx, c, rootPath, rootPlugin, nil); err != nil {
		return pkgerrors.Wrap(err, "failed to run test")
	}
	return nil
}

// runner performs a test by running a set of plugins defined in a config file.
type runner struct {
	// plugins registered for this runner.
	plugins map[string]any

	// config fragments stored during the parseConfig operation, so they can be retrieved later at run stage.
	configs map[string]any

	// prompter defines a function to be called to prompt users.
	prompter func(context.Context, string)
}

func newRunner(plugins map[string]any) *runner {
	return &runner{
		plugins: plugins,
		configs: make(map[string]any),
	}
}

// parseConfig parses the config file passed to the runner as input.
// Note: during this operation, config fragments are stored in runner.configs so they can be retrieved later at run stage.
func (r *runner) parseConfig(ctx context.Context, path *field.Path, plugin any, callStack []string, rawConfig []byte) error {
	// TODO: in future we might want to injecting env variables / use templating

	var config map[string]any
	if err := yaml.Unmarshal(rawConfig, &config); err != nil {
		return pkgerrors.Wrapf(err, "failed to unmarshal config for %s", path.String())
	}

	// If this is a plugin that validates its call stack, perform this validation.
	if validator, ok := plugin.(CallStackValidatorPlugin); ok {
		if err := validator.ValidateCallStack(ctx, callStack); err != nil {
			return pkgerrors.Wrapf(err, "failed to validate call stack for %s", path.String())
		}
	}

	// If this is a configurable plugin, read the custom plugin config.
	// Note: When invoking plugin's ParseConfig method, drop all the fields that are managed from the runner itself,
	// so the plugin authors can use Unmarshal strict.
	pluginConfig := maps.Clone(config)
	maps.DeleteFunc(pluginConfig, func(k string, _ any) bool {
		return k == "run" || k == "runAfter" || k == "runOnError" || k == "runConfig" || k == "plugin"
	})

	if configurable, ok := plugin.(ConfigurablePlugin); ok {
		var err error
		var rawPluginConfig []byte
		if rawPluginConfig, err = yaml.Marshal(pluginConfig); err != nil {
			return pkgerrors.Wrapf(err, "failed to marshal plugin config for %s", path.String())
		}

		c, err := configurable.ParseConfig(ctx, rawPluginConfig)
		if err != nil {
			return pkgerrors.Wrapf(err, "failed to parse plugin config for %s", path.String())
		}

		// Store the configuration for this plugin using <path> as key, so it can be retrieved later.
		key := path.String()
		r.configs[key] = c
	} else if len(pluginConfig) > 0 {
		return pkgerrors.Errorf("plugin %s has a custom config, but the plugin cannot parse it", path.String())
	}

	// If this is an imported plugin, merge the config with the imported one.
	if importer, ok := plugin.(*importerPlugin); ok {
		key := path.String()

		var err error
		config, err = importer.MergeConfig(ctx, config, r.configs[key])
		if err != nil {
			return pkgerrors.Errorf("failed to import config for field %s", path.String())
		}
		delete(r.configs, key)
	}

	// All plugin could have a runConfig field, parse it.
	// Note: even if runConfig field is not defined, a default runConfig is returned.
	runConfig, err := r.getRunConfig(ctx, path, config)
	if err != nil {
		return pkgerrors.Wrapf(err, "failed to parse %s.runConfig", path.String())
	}
	// Store the runConfig using <path>.runConfig as key, so it can be retrieved later.
	key := path.Child("runConfig").String()
	r.configs[key] = runConfig

	// if this is a selector plugin, the config must have a run field defining which plugin to run for the selected objects
	// and corresponding config.
	if _, ok := plugin.(SelectorPlugin); ok {
		if err := r.parseRunList(ctx, "run", path, config, callStack); err != nil {
			return err
		}
	}

	// if the plugin has a runOnError list. Parse it.
	if _, ok := config["runOnError"]; ok {
		if err := r.parseRunList(ctx, "runOnError", path, config, callStack); err != nil {
			return err
		}
	}

	// if the plugin has a runAfter list. Parse it.
	if _, ok := config["runAfter"]; ok {
		if err := r.parseRunList(ctx, "runAfter", path, config, callStack); err != nil {
			return err
		}
	}

	return nil
}

func (r *runner) parseRunList(ctx context.Context, field string, path *field.Path, config map[string]any, callStack []string) error {
	runList, err := r.getRunList(ctx, field, path, config)
	if err != nil {
		return pkgerrors.Wrapf(err, "failed to parse %s", path.String())
	}
	// Store the run list using <path>.<field> as key, so it can be retrieved later.
	key := path.Child(field).String()
	r.configs[key] = runList

	// Go through all the plugins in the runConfig, and read/validate corresponding config.
	for i, item := range runList {
		// Use <path>.<field>[n] as a key for each run config, so it can be retrieved later.
		itemPath := path.Child(field).Index(i)
		itemRawConfig := item.RawConfig
		if err := r.parseConfig(ctx, itemPath, item.Plugin, append(callStack, item.PluginName), itemRawConfig); err != nil {
			return err
		}
	}
	return nil
}

// runOne runs a plugin for the given TestObjects.
func (r *runner) runOne(ctx context.Context, c client.Client, path *field.Path, plugin any, objects TestObjects) (retErr error) {
	// Get the runConfig that was previously stored using <path>.runConfig as key.
	key := path.Child("runConfig").String()
	runConfig, ok := r.configs[key].(*RunConfig)
	if !ok {
		// Note: this should never happen because runConfig are automatically generated by readAndValidateConfig.
		return pkgerrors.Errorf("trying to access %s.runConfig", path)
	}

	// Get the plugin config that was previously stored using <path> as key.
	// Note: not all the plugins have a config.
	key = path.String()
	pluginConfig := r.configs[key]

	// If running in debug mode, prompt before running the plugin.
	if ptr.Deref(runConfig.Debug, false) {
		r.prompt(ctx, "Debug on", ", press enter to continue", path, plugin, objects, pluginConfig)
	}

	// If a timeout applies to the plugin call, set the context accordingly.
	if runConfig.Timeout != nil {
		cancelCtx, cancelFunc := context.WithTimeout(ctx, runConfig.Timeout.Duration)
		defer cancelFunc()
		ctx = cancelCtx
	}

	// Set up a defer to run runOnError and runAfter plugins.
	defer func() {
		var cleanupErrs []error

		if retErr != nil {
			key = path.Child("runOnError").String()
			if runList, ok := r.configs[key].([]runListItem); ok {
				for i, config := range runList {
					// Run the plugin, using <path>.runOnError[n] as a path, thus making it possible to fetch the corresponding config.
					pluginPath := path.Child("runOnError").Index(i)
					if err := r.runOne(ctx, c, pluginPath, config.Plugin, objects); err != nil {
						cleanupErrs = append(cleanupErrs, pkgerrors.Wrapf(err, "failed to run %s", pluginPath.String()))
						break
					}
				}
			}
		}

		key = path.Child("runAfter").String()
		if runList, ok := r.configs[key].([]runListItem); ok {
			for i, config := range runList {
				// Run the plugin, using <path>.runAfter[n] as a path, thus making it possible to fetch the corresponding config.
				pluginPath := path.Child("runAfter").Index(i)
				if err := r.runOne(ctx, c, pluginPath, config.Plugin, objects); err != nil {
					cleanupErrs = append(cleanupErrs, pkgerrors.Wrapf(err, "failed to run %s", pluginPath.String()))
					break
				}
			}
		}

		if err := kerrors.NewAggregate(cleanupErrs); err != nil {
			retErr = kerrors.NewAggregate([]error{retErr, err})
		}
	}()

	// if this is a selector plugin, invoke the Select method and then trigger runMany for the resulting objects.
	if selector, ok := plugin.(SelectorPlugin); ok {
		objectsList, err := selector.Select(ctx, c, objects, pluginConfig, *runConfig)
		if err != nil {
			// If running in debugOnError mode, prompt after the plugin returned an error
			if ptr.Deref(runConfig.DebugOnError, false) {
				r.prompt(ctx, "Debug error on", ", press enter to continue", path, plugin, objects, pluginConfig)
			}
			return pkgerrors.Wrapf(err, "failed to select objects for %s", path.String())
		}

		// Get the runList that was previously stored using <path>.run as key.
		key = path.Child("run").String()
		runList, ok := r.configs[key].([]runListItem)
		if !ok {
			// Note: this should never happen because runList are automatically generated by readAndValidateConfig.
			return pkgerrors.Errorf("trying to access %s.run", path)
		}

		if err := r.runMany(ctx, c, path.Child("run"), runList, runConfig, objectsList); err != nil {
			return err
		}
	}

	// if this is an executor plugin, invoke the Exec method.
	if executor, ok := plugin.(ExecutorPlugin); ok {
		if err := executor.Exec(ctx, c, objects, pluginConfig, *runConfig); err != nil {
			// If running in debugOnError mode, prompt after the plugin returned an error
			if ptr.Deref(runConfig.DebugOnError, false) {
				r.prompt(ctx, "Debug error on", ", press enter to continue", path, plugin, objects, pluginConfig)
			}
			return pkgerrors.Wrapf(err, "failed to exec %s", path.String())
		}
	}

	return nil
}

// runMany runs a list of plugins for a list of TestObjects.
func (r *runner) runMany(ctx context.Context, c client.Client, path *field.Path, runList []runListItem, runConfig *RunConfig, testObjectsList []TestObjects) error {
	wg := &sync.WaitGroup{}
	inputChan := make(chan TestObjects)
	doneChan := make(chan bool)
	errorChan := make(chan error)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start as many workers as defined in runConfig.Concurrency.
	concurrency := ptr.Deref(runConfig.Concurrency, 1)
	if concurrency < 1 {
		return pkgerrors.New("concurrency must be greater than zero")
	}
	for range concurrency {
		wg.Add(1)
		go func(ctx context.Context, inputChan chan TestObjects, errChan chan error, wg *sync.WaitGroup) {
			defer wg.Done()

			for {
				done := func() bool {
					select {
					case <-ctx.Done():
						// If the context is canceled, return and shutdown the worker.
						return true
					case input, open := <-inputChan:
						// If the channel is closed it implies there is no more work to be done. return and shutdown the worker.
						if !open {
							return true
						}

						// For each input object, exec all the plugins defined in the runConfig.
						for i, config := range runList {
							// Run the plugin, using <path>.run[n] as a path, thus making it possible to fetch the corresponding config.
							pluginPath := path.Index(i)
							if err := r.runOne(ctx, c, pluginPath, config.Plugin, input); err != nil {
								errChan <- err
								break
							}
						}
						return false
					}
				}()
				if done {
					break
				}
			}
		}(ctx, inputChan, errorChan, wg)
	}

	// Add as many TestObjects as defined by runConfig.Limit into the input channel.
	go func() {
		for i, v := range testObjectsList {
			if i >= int(ptr.Deref(runConfig.Limit, math.MaxInt32)) {
				break
			}
			inputChan <- v
		}
		close(inputChan)
	}()

	// Wait for processing to complete.
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	errs := []error{}

outer:
	// Wait for completion, collect errors if any.
	for {
		select {
		case err := <-errorChan:
			errs = append(errs, err)
			if ptr.Deref(runConfig.FailFast, false) {
				cancel()
			}
		case <-doneChan:
			break outer
		}
	}

	close(errorChan)
	return kerrors.NewAggregate(errs)
}

// prompt calls the prompter, if configured, with a message describing the given plugin call.
func (r *runner) prompt(ctx context.Context, msgPrefix, msgSuffix string, path *field.Path, plugin any, objects TestObjects, pluginConfig any) {
	if r.prompter == nil {
		return
	}

	if _, ok := plugin.(*importerPlugin); ok {
		return
	}

	msg := path.String()
	if generator, ok := plugin.(MessageGeneratorPlugin); ok {
		if generatedMessage, err := generator.GenerateMessage(objects, pluginConfig); err == nil {
			msg = generatedMessage
		}
	}
	r.prompter(ctx, fmt.Sprintf("%s %s%s", msgPrefix, msg, msgSuffix))
}

// getRunConfig parses a runConfig field if defined, otherwise returns a default RunConfig.
func (r *runner) getRunConfig(_ context.Context, path *field.Path, rawConfig map[string]any) (*RunConfig, error) {
	// Get the runConfig.
	runConfig := &RunConfig{}
	if runConfigRaw, ok := rawConfig["runConfig"].(map[string]any); ok {
		cfgByte, err := yaml.Marshal(runConfigRaw)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to marshal %s.runConfig", path)
		}

		if err := yaml.UnmarshalStrict(cfgByte, &runConfig); err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to unmarshal %s.runConfig", path)
		}
	}

	// Use top level runConfig as a default when it makes sense.
	if path.String() != rootPath.String() {
		key := rootPath.Child("runConfig").String()
		rootRunConfig, ok := r.configs[key].(*RunConfig)
		if !ok {
			// Note: this should never happen because runConfig are automatically generated by readAndValidateConfig.
			return nil, pkgerrors.Errorf("trying to access %s.runConfig", rootPath)
		}

		if runConfig.DryRun == nil {
			runConfig.DryRun = rootRunConfig.DryRun
		}

		if runConfig.Debug == nil {
			runConfig.Debug = rootRunConfig.Debug
		}

		if runConfig.DebugOnError == nil {
			runConfig.DebugOnError = rootRunConfig.DebugOnError
		}

		for k, v := range rootRunConfig.Intervals {
			if _, ok := runConfig.Intervals[k]; ok {
				continue
			}
			if runConfig.Intervals == nil {
				runConfig.Intervals = make(map[string][]metav1.Duration)
			}
			runConfig.Intervals[k] = v
		}
	}

	return runConfig, nil
}

type runListItem struct {
	// PluginName is the name of the plugin to be executed.
	PluginName string

	// Plugin is the plugin to be executed.
	Plugin any

	// RawConfig is the config for this plugin.
	RawConfig []byte
}

// getRunList parses a run field.
func (r *runner) getRunList(_ context.Context, field string, path *field.Path, rawConfig map[string]any) ([]runListItem, error) {
	// Get the List of plugin to run and the corresponding config.
	runList := make([]runListItem, 0)

	_, ok := rawConfig[field]
	if !ok {
		return nil, pkgerrors.Errorf("field %s.%s is missing", path, field)
	}

	runListRaw, ok := rawConfig[field].([]any)
	if !ok {
		return nil, pkgerrors.Errorf("field %s.%s must be a list", path, field)
	}

	if len(runListRaw) == 0 {
		return nil, pkgerrors.Errorf("field %s.%s must have at least one item", path, field)
	}

	for i, task := range runListRaw {
		runItemConfig, ok := task.(map[string]any)
		if !ok {
			return nil, pkgerrors.Errorf("%s.%s[%d] must be an object", path, field, i)
		}

		pluginName, ok := runItemConfig["plugin"].(string)
		if !ok {
			return nil, pkgerrors.Errorf("field %s.%s[%d].plugin is missing or it is not a valid string", path, field, i)
		}

		plugin, ok := r.plugins[pluginName]
		if !ok {
			plugins := slices.Collect(maps.Keys(r.plugins))
			sort.Strings(plugins)
			return nil, pkgerrors.Errorf("field %s.%s[%d].plugin must be one of [%s]", path, field, i, strings.Join(plugins, ", "))
		}

		delete(runItemConfig, "plugin")
		cfgByte, err := yaml.Marshal(runItemConfig)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to marshal %s.%s[%d]", path, field, i)
		}

		runList = append(runList, runListItem{
			PluginName: pluginName,
			Plugin:     plugin,
			RawConfig:  cfgByte,
		})
	}

	return runList, nil
}
