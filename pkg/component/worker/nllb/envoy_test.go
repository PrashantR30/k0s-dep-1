/*
Copyright 2022 k0s authors

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

package nllb

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	k0snet "github.com/k0sproject/k0s/internal/pkg/net"

	"k8s.io/client-go/util/jsonpath"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestWriteEnvoyConfig(t *testing.T) {
	for _, test := range []struct {
		name     string
		expected int
		servers  []string
	}{
		{"empty", 0, []string{}},
		{"one", 1, []string{"foo:16"}},
		{"two", 2, []string{"foo:16", "bar:17"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			params := envoyParams{
				configDir: dir,
				bindIP:    net.IPv6loopback,
			}
			filesParams := envoyFilesParams{}
			for _, server := range test.servers {
				server, err := k0snet.ParseHostPort(server)
				require.NoError(t, err)
				filesParams.apiServers = append(filesParams.apiServers, *server)
			}

			require.NoError(t, writeEnvoyConfig(&params, &filesParams))

			content, err := os.ReadFile(filepath.Join(dir, "envoy.yaml"))
			require.NoError(t, err)
			var parsed map[string]any
			require.NoError(t, yaml.Unmarshal(content, &parsed), "invalid YAML in envoy.yaml")

			if ip, err := evalJSONPath[string](parsed,
				".static_resources.listeners[0].address.socket_address.address",
			); assert.NoError(t, err) {
				assert.Equal(t, "::1", ip)
			}
		})
	}
}

func evalJSONPath[T any](json any, path string) (t T, _ error) {
	tpl := jsonpath.New("")
	if err := tpl.Parse("{" + path + "}"); err != nil {
		return t, err
	}

	results, err := tpl.FindResults(json)
	switch {
	case err != nil:
		return t, err
	case len(results) == 0:
		return t, errors.New("given jsonpath expression does not match any value")
	case len(results) > 1:
		return t, errors.New("given jsonpath expression matches more than one list")
	case len(results[0]) > 1:
		return t, errors.New("given jsonpath expression matches more than one value")
	}

	candidate := results[0][0].Interface()
	converted, ok := candidate.(T)
	if !ok {
		return t, fmt.Errorf("expected %T, found %T", t, candidate)
	}

	return converted, nil
}
