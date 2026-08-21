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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

func Test_runner_parseConfig(t *testing.T) {
	g := NewWithT(t)

	importConfig := map[string]any{
		"run": []any{
			map[string]any{
				"plugin": testExecutorPluginKey,
				"foo":    "123",
			},
		},
	}

	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.yaml")
	tempFileConfig, err := yaml.Marshal(importConfig)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(tempFile, tempFileConfig, 0600)
	g.Expect(err).ToNot(HaveOccurred())

	importer := &importerPlugin{}
	testExecutor := &testExecutorPlugin{}
	testSelector := &testSelectorPlugin{}
	tests := []struct {
		name           string
		config         map[string]any
		wantErrMessage string
		wantConfigs    map[string]any
	}{
		{
			name:           "Empty config",
			config:         nil,
			wantErrMessage: "failed to parse $: field $.run is missing",
		},
		{
			name:           "Config without .run",
			config:         make(map[string]any),
			wantErrMessage: "failed to parse $: field $.run is missing",
		},
		{
			name: "Config with .run not as a list",
			config: map[string]any{
				"run": "foo",
			},
			wantErrMessage: "failed to parse $: field $.run must be a list",
		},
		{
			name: "Config with  .run items not an object",
			config: map[string]any{
				"run": []any{
					"foo",
				},
			},
			wantErrMessage: "failed to parse $: $.run[0] must be an object",
		},
		{
			name: "Config with .run items without plugin field",
			config: map[string]any{
				"run": []any{
					map[string]any{},
				},
			},
			wantErrMessage: "failed to parse $: field $.run[0].plugin is missing or it is not a valid string",
		},
		{
			name: "Config with .run items with plugin of the wrong type",
			config: map[string]any{
				"run": []any{
					map[string]any{
						"plugin": map[string]any{},
					},
				},
			},
			wantErrMessage: "failed to parse $: field $.run[0].plugin is missing or it is not a valid string",
		},
		{
			name: "Config with .run items with unknown plugin",
			config: map[string]any{
				"run": []any{
					map[string]any{
						"plugin": "unknown",
					},
				},
			},
			wantErrMessage: fmt.Sprintf("failed to parse $: field $.run[0].plugin must be one of [%s, test.executor, test.selector]", importerPluginKey),
		},
		{
			name: "Config with .run executor items with wrong config",
			config: map[string]any{
				"run": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"bar":    "",
					},
				},
			},
			wantErrMessage: "failed to parse plugin config for $.run[0]: error unmarshaling JSON: while decoding JSON: json: unknown field \"bar\"",
		},
		{
			name: "Config failing CallStack validation",
			config: map[string]any{
				"run": []any{
					map[string]any{
						"plugin": testSelectorPluginKey,
						"run": []any{
							map[string]any{
								"plugin": testSelectorPluginKey,
							},
						},
					},
				},
			},
			wantErrMessage: "failed to validate call stack for $.run[0].run[0]: this plugin can be used only at top level",
		},
		{
			name: "Config with .run executor items with correct config",
			config: map[string]any{
				"run": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "123",
					},
				},
			},
			wantErrMessage: "", // No error
			wantConfigs: map[string]any{
				"$.runConfig": &RunConfig{},
				"$.run": []runListItem{
					{
						PluginName: testExecutorPluginKey,
						Plugin:     testExecutor,
						RawConfig:  []byte("foo: \"123\"\x0A"),
					},
				},
				"$.run[0].runConfig": &RunConfig{},
				"$.run[0]":           &testExecutorPluginConfig{Foo: "123"},
			},
		},
		{
			name: "Config with .run selector items with wrong config",
			config: map[string]any{
				"run": []any{
					map[string]any{
						"plugin": testSelectorPluginKey,
						// run is missing
					},
				},
			},
			wantErrMessage: "failed to parse $.run[0]: field $.run[0].run is missing",
		},
		{
			name: "Config with .run selector items with correct config",
			config: map[string]any{
				"run": []any{
					map[string]any{
						"plugin": testSelectorPluginKey,
						"run": []any{
							map[string]any{
								"plugin": testExecutorPluginKey,
								"foo":    "123",
							},
						},
					},
				},
			},
			wantErrMessage: "", // No error
			wantConfigs: map[string]any{
				"$.runConfig": &RunConfig{},
				"$.run": []runListItem{
					{
						PluginName: testSelectorPluginKey,
						Plugin:     testSelector,
						RawConfig:  []byte("run:\n- foo: \"123\"\n  plugin: test.executor\n"),
					},
				},
				"$.run[0].runConfig": &RunConfig{},
				"$.run[0].run": []runListItem{
					{
						PluginName: testExecutorPluginKey,
						Plugin:     testExecutor,
						RawConfig:  []byte("foo: \"123\"\x0A"),
					},
				},
				"$.run[0].run[0].runConfig": &RunConfig{},
				"$.run[0].run[0]":           &testExecutorPluginConfig{Foo: "123"},
			},
		},
		{
			name: "Import config",
			config: map[string]any{
				"run": []any{
					map[string]any{
						"plugin": importerPluginKey,
						"path":   tempFile,
					},
				},
			},
			wantErrMessage: "", // No error
			wantConfigs: map[string]any{
				"$.runConfig": &RunConfig{},
				"$.run": []runListItem{
					{
						PluginName: importerPluginKey,
						Plugin:     importer,
						RawConfig:  []byte(fmt.Sprintf("path: %s\x0A", tempFile)),
					},
				},
				"$.run[0].runConfig": &RunConfig{},
				"$.run[0].run": []runListItem{
					{
						PluginName: testExecutorPluginKey,
						Plugin:     testExecutor,
						RawConfig:  []byte("foo: \"123\"\x0A"),
					},
				},
				"$.run[0].run[0].runConfig": &RunConfig{},
				"$.run[0].run[0]":           &testExecutorPluginConfig{Foo: "123"},
			},
		},
		{
			name: "Run after",
			config: map[string]any{
				"run": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "123",
					},
				},
				"runAfter": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "123",
					},
				},
			},
			wantErrMessage: "", // No error
			wantConfigs: map[string]any{
				"$.runConfig": &RunConfig{},
				"$.run": []runListItem{
					{
						PluginName: testExecutorPluginKey,
						Plugin:     testExecutor,
						RawConfig:  []byte("foo: \"123\"\x0A"),
					},
				},
				"$.run[0].runConfig": &RunConfig{},
				"$.run[0]":           &testExecutorPluginConfig{Foo: "123"},
				"$.runAfter": []runListItem{
					{
						PluginName: testExecutorPluginKey,
						Plugin:     testExecutor,
						RawConfig:  []byte("foo: \"123\"\x0A"),
					},
				},
				"$.runAfter[0].runConfig": &RunConfig{},
				"$.runAfter[0]":           &testExecutorPluginConfig{Foo: "123"},
			},
		},
		{
			name: "Run on error",
			config: map[string]any{
				"run": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "123",
					},
				},
				"runOnError": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "123",
					},
				},
			},
			wantErrMessage: "", // No error
			wantConfigs: map[string]any{
				"$.runConfig": &RunConfig{},
				"$.run": []runListItem{
					{
						PluginName: testExecutorPluginKey,
						Plugin:     testExecutor,
						RawConfig:  []byte("foo: \"123\"\x0A"),
					},
				},
				"$.run[0].runConfig": &RunConfig{},
				"$.run[0]":           &testExecutorPluginConfig{Foo: "123"},
				"$.runOnError": []runListItem{
					{
						PluginName: testExecutorPluginKey,
						Plugin:     testExecutor,
						RawConfig:  []byte("foo: \"123\"\x0A"),
					},
				},
				"$.runOnError[0].runConfig": &RunConfig{},
				"$.runOnError[0]":           &testExecutorPluginConfig{Foo: "123"},
			},
		},
		// TODO: runConfig validation
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			r := newRunner(
				map[string]any{
					importerPluginKey:     importer,
					testExecutorPluginKey: testExecutor,
					testSelectorPluginKey: testSelector,
				},
			)

			rawConfig, err := yaml.Marshal(tt.config)
			g.Expect(err).ToNot(HaveOccurred())

			err = r.parseConfig(t.Context(), rootPath, &rootSelectorPlugin{}, nil, rawConfig)
			if tt.wantErrMessage != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tt.wantErrMessage))
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(r.configs).To(Equal(tt.wantConfigs))
		})
	}
}

func TestRun(t *testing.T) {
	g := NewWithT(t)

	importConfig := map[string]any{
		"run": []any{
			map[string]any{
				"plugin": testExecutorPluginKey,
				"foo":    "123",
			},
		},
	}

	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.yaml")
	tempFileConfig, err := yaml.Marshal(importConfig)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(tempFile, tempFileConfig, 0600)
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		name           string
		config         map[string]any
		wantCalls      []string
		wantPrompts    []string
		wantErr        bool
		wantErrMessage string
	}{
		{
			name: "Run an executor",
			config: map[string]any{
				"plugin": testExecutorPluginKey,
				"foo":    "123",
			},
			wantCalls: []string{
				"exec a=vA, foo=123",
			},
			wantErr: false,
		},
		{
			name: "Run a selector, for each item run an executor",
			config: map[string]any{
				"plugin": testSelectorPluginKey,
				"run": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "123",
					},
				},
			},
			wantCalls: []string{
				"select a=vA",
				"exec a=vA, foo=123, k1=v0",
				"exec a=vA, foo=123, k1=v1",
				"exec a=vA, foo=123, k1=v2",
				"exec a=vA, foo=123, k1=v3",
				"exec a=vA, foo=123, k1=v4",
			},
			wantErr: false,
		},
		{
			name: "Run a selector, for each item run an executor, limit 3",
			config: map[string]any{
				"plugin": testSelectorPluginKey,
				"runConfig": map[string]any{
					"limit": 3,
				},
				"run": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "123",
					},
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "456",
					},
				},
			},
			wantCalls: []string{
				"select a=vA",
				"exec a=vA, foo=456, k1=v0",
				"exec a=vA, foo=123, k1=v0",
				"exec a=vA, foo=123, k1=v1",
				"exec a=vA, foo=456, k1=v1",
				"exec a=vA, foo=123, k1=v2",
				"exec a=vA, foo=456, k1=v2",
			},
			wantErr: false,
		},
		{
			name: "runAfter top level",
			config: map[string]any{
				"plugin": testSelectorPluginKey,
				"runConfig": map[string]any{
					"limit": 1,
				},
				"run": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "123",
					},
				},
				"runAfter": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "after",
					},
				},
				"runOnError": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "onError",
					},
				},
			},
			wantCalls: []string{
				"select a=vA",
				"exec a=vA, foo=123, k1=v0",
				"exec a=vA, foo=after",
			},
			wantErr: false,
		},
		{
			name: "runAfter on a specific plugin",
			config: map[string]any{
				"plugin": testSelectorPluginKey,
				"runConfig": map[string]any{
					"limit": 1,
				},
				"run": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "123",
						"runAfter": []any{
							map[string]any{
								"plugin": testExecutorPluginKey,
								"foo":    "after",
							},
						},
						"runOnError": []any{
							map[string]any{
								"plugin": testExecutorPluginKey,
								"foo":    "onError",
							},
						},
					},
				},
			},
			wantCalls: []string{
				"select a=vA",
				"exec a=vA, foo=123, k1=v0",
				"exec a=vA, foo=after, k1=v0",
			},
			wantErr: false,
		},
		{
			name: "runOnError top level",
			config: map[string]any{
				"plugin": testSelectorPluginKey,
				"runConfig": map[string]any{
					"limit": 1,
				},
				"run": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "123",
						"fail":   true,
					},
				},
				"runAfter": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "after",
					},
				},
				"runOnError": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "onError",
					},
				},
			},
			wantCalls: []string{
				"select a=vA",
				"exec a=vA, foo=123, k1=v0",
				"exec a=vA, foo=onError",
				"exec a=vA, foo=after",
			},
			wantErr: true,
		},
		{
			name: "runOnError on a specific plugin",
			config: map[string]any{
				"plugin": testSelectorPluginKey,
				"runConfig": map[string]any{
					"limit": 1,
				},
				"run": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "123",
						"fail":   true,
						"runAfter": []any{
							map[string]any{
								"plugin": testExecutorPluginKey,
								"foo":    "after",
							},
						},
						"runOnError": []any{
							map[string]any{
								"plugin": testExecutorPluginKey,
								"foo":    "onError",
							},
						},
					},
				},
			},
			wantCalls: []string{
				"select a=vA",
				"exec a=vA, foo=123, k1=v0",
				"exec a=vA, foo=onError, k1=v0",
				"exec a=vA, foo=after, k1=v0",
			},
			wantErr: true,
		},
		{
			name: "runOnError plugin itself fails",
			config: map[string]any{
				"plugin": testSelectorPluginKey,
				"runConfig": map[string]any{
					"limit": 1,
				},
				"run": []any{
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "123",
						"fail":   true,
						"runOnError": []any{
							map[string]any{
								"plugin": testExecutorPluginKey,
								"foo":    "onError",
								"fail":   true,
							},
						},
					},
				},
			},
			wantCalls: []string{
				"select a=vA",
				"exec a=vA, foo=123, k1=v0",
				"exec a=vA, foo=onError, k1=v0",
			},
			wantErr:        true,
			wantErrMessage: "[failed to exec $.run[0]: fail, failed to run $.run[0].runOnError[0]: failed to exec $.run[0].runOnError[0]: fail]",
		},
		{
			name: "Run imported config",
			config: map[string]any{
				"plugin": testSelectorPluginKey,
				"runConfig": map[string]any{
					"limit": 1,
				},
				"run": []any{
					map[string]any{
						"plugin": importerPluginKey, // resolves to test.executor, foo=123
						"path":   tempFile,
					},
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "456",
					},
				},
			},
			wantCalls: []string{
				"select a=vA",
				"exec a=vA, foo=456, k1=v0",
				"exec a=vA, foo=123, k1=v0",
			},
			wantErr: false,
		},
		{
			name: "Debug top level",
			config: map[string]any{
				"plugin": testSelectorPluginKey,
				"runConfig": map[string]any{
					"limit": 1,
					"debug": true,
				},
				"run": []any{
					map[string]any{
						"plugin": importerPluginKey, // resolves to test.executor, foo=123
						"path":   tempFile,
					},
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "456",
					},
				},
			},
			wantCalls: []string{
				"select a=vA",
				"exec a=vA, foo=456, k1=v0",
				"exec a=vA, foo=123, k1=v0",
			},
			wantPrompts: []string{
				"Debug:  Test selector, press enter to continue",
				"Debug:  Test executor a=vA, foo=123, k1=v0, press enter to continue",
				"Debug:  Test executor a=vA, foo=456, k1=v0, press enter to continue",
			},
			wantErr: false,
		},
		{
			name: "Debug on a specific plugin",
			config: map[string]any{
				"plugin": testSelectorPluginKey,
				"runConfig": map[string]any{
					"limit": 1,
				},
				"run": []any{
					map[string]any{
						"plugin": importerPluginKey, // resolves to test.executor, foo=123
						"path":   tempFile,
					},
					map[string]any{
						"plugin": testExecutorPluginKey,
						"foo":    "456",
						"runConfig": map[string]any{
							"debug": true,
						},
					},
				},
			},
			wantCalls: []string{
				"select a=vA",
				"exec a=vA, foo=456, k1=v0",
				"exec a=vA, foo=123, k1=v0",
			},
			wantPrompts: []string{
				"Debug:  Test executor a=vA, foo=456, k1=v0, press enter to continue",
			},
			wantErr: false,
		},
		{
			name: "Debug On Error",
			config: map[string]any{
				"plugin": testExecutorPluginKey,
				"fail":   true,
				"foo":    "123",
				"runConfig": map[string]any{
					"debugOnError": true,
				},
			},
			wantCalls: []string{
				"exec a=vA, foo=123",
			},
			wantPrompts: []string{
				"Error runningTest executor a=vA, foo=123, press enter to continue",
			},
			wantErr: true,
		},
		// TODO: check timeout
		// TODO: check error management (on a sequence of plugins, on a list of objects)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			calls := []string{}
			logger := func(msg string) {
				calls = append(calls, msg)
			}

			r := newRunner(
				map[string]any{
					importerPluginKey:     &importerPlugin{},
					testExecutorPluginKey: &testExecutorPlugin{logger: logger},
					testSelectorPluginKey: &testSelectorPlugin{logger: logger},
				},
			)

			prompts := []string{}
			r.prompter = func(_ context.Context, msg string) {
				prompts = append(prompts, msg)
			}

			pluginName := tt.config["plugin"].(string)
			plugin := r.plugins[pluginName]
			g.Expect(plugin).ToNot(BeNil())

			rawConfig, err := yaml.Marshal(tt.config)
			g.Expect(err).ToNot(HaveOccurred())

			err = r.parseConfig(t.Context(), rootPath, plugin, nil, rawConfig)
			g.Expect(err).ToNot(HaveOccurred())

			err = r.runOne(t.Context(), nil, rootPath, plugin, TestObjects{"a": "vA"})
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				if tt.wantErrMessage != "" {
					g.Expect(err.Error()).To(Equal(tt.wantErrMessage))
				}
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			g.Expect(calls).To(ConsistOf(tt.wantCalls))
			g.Expect(prompts).To(ConsistOf(tt.wantPrompts))
		})
	}
}

type testExecutorPlugin struct {
	logger func(string)
}

var _ MessageGeneratorPlugin = &testExecutorPlugin{}

// GenerateMessage generate a message text for the plugin call.
func (p testExecutorPlugin) GenerateMessage(objects TestObjects, pluginConfigUntyped any) (string, error) {
	return fmt.Sprintf("Test executor %s", strings.Join(logKeys(objects, pluginConfigUntyped), ", ")), nil
}

var _ ConfigurablePlugin = &testExecutorPlugin{}

type testExecutorPluginConfig struct {
	Foo  string `json:"foo"`
	Fail bool   `json:"fail"`
}

func (p testExecutorPlugin) ParseConfig(_ context.Context, rawConfig []byte) (any, error) {
	config := &testExecutorPluginConfig{}
	if err := yaml.UnmarshalStrict(rawConfig, config); err != nil {
		return nil, err
	}
	return config, nil
}

const testExecutorPluginKey = "test.executor"

var _ ExecutorPlugin = &testExecutorPlugin{}

func (p testExecutorPlugin) Exec(_ context.Context, _ client.Client, objects TestObjects, pluginConfigUntyped any, _ RunConfig) error {
	config := &testExecutorPluginConfig{}
	if pluginConfigUntyped != nil {
		config = pluginConfigUntyped.(*testExecutorPluginConfig)
	}

	// Log the call, surfacing all the objects, and the config.
	p.logger(fmt.Sprintf("exec %s", strings.Join(logKeys(objects, pluginConfigUntyped), ", ")))

	if config.Fail {
		return errors.New("fail")
	}
	return nil
}

const testSelectorPluginKey = "test.selector"

type testSelectorPlugin struct {
	logger func(string)
}

var _ MessageGeneratorPlugin = &testSelectorPlugin{}

func (p testSelectorPlugin) GenerateMessage(_ TestObjects, _ any) (string, error) {
	return "Test selector", nil
}

var _ CallStackValidatorPlugin = &testSelectorPlugin{}

func (p testSelectorPlugin) ValidateCallStack(_ context.Context, callStack []string) error {
	if len(callStack) > 1 {
		return errors.New("this plugin can be used only at top level")
	}
	return nil
}

var _ SelectorPlugin = &testSelectorPlugin{}

func (p testSelectorPlugin) Select(_ context.Context, _ client.Client, objects TestObjects, pluginConfigUntyped any, _ RunConfig) ([]TestObjects, error) {
	// Log the call, surfacing all the objects, and the config.
	p.logger(fmt.Sprintf("select %s", strings.Join(logKeys(objects, pluginConfigUntyped), ", ")))

	// return 5 objects with the same kv in objects + a new one.
	ret := []TestObjects{} //nolint:prealloc
	for i := range 5 {
		t := TestObjects{}
		for k, v := range objects {
			t[k] = v
		}
		t[fmt.Sprintf("k%d", len(t))] = fmt.Sprintf("v%d", i)

		ret = append(ret, t)
	}
	return ret, nil
}

func logKeys(objects TestObjects, pluginConfigUntyped any) []string {
	kv := []string{}
	for k, v := range objects {
		kv = append(kv, fmt.Sprintf("%s=%s", k, v))
	}
	if pluginConfigUntyped != nil {
		config := pluginConfigUntyped.(*testExecutorPluginConfig)
		kv = append(kv, fmt.Sprintf("foo=%s", config.Foo))
	}
	sort.Strings(kv)
	return kv
}
