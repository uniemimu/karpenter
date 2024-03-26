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

package provisioning

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/logging"

	"sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
)

type InstanceTypeExtender struct {
	solvedMatches map[string]map[string]*extension // nodepoolkey -> instancetypename -> extended resources OR nil
	extensions    []*extension
}

type extensionJson struct {
	InstanceNameRegex      string            `json:"instanceNameRegex"`
	ExtendedResources      map[string]int64  `json:"extendedResources"`
	RequiredNodePoolLabels map[string]string `json:"requiredNodePoolLabels,omitempty"`
}

type extension struct {
	requiredNPLabels  map[string]string
	regex             *regexp.Regexp
	extendedResources v1.ResourceList
}

var (
	extOnce  sync.Once
	extender *InstanceTypeExtender
)

func getKey(np *v1beta1.NodePool) string {
	return np.Name + "&" + np.Namespace
}

func readExtensions(ctx context.Context) []extensionJson {
	extensions := []extensionJson{}

	// read file
	data, err := os.ReadFile("/extensions/instance_type_extensions.json")
	if err != nil {
		logging.FromContext(ctx).Errorf("failed reading extensions, err: %s", err)

		return nil
	}

	err = json.Unmarshal(data, &extensions)
	if err != nil {
		logging.FromContext(ctx).Errorf("failed parsing json, err: %s", err)

		return nil
	}

	return extensions
}

func (e *InstanceTypeExtender) init(ctx context.Context) {
	extensionList := readExtensions(ctx)

	// TODO validate extensionList content

	if extensionList == nil {
		return
	}

	for _, instanceTypeExtension := range extensionList {
		regex, err := regexp.Compile(instanceTypeExtension.InstanceNameRegex)
		if err != nil {
			logging.FromContext(ctx).Errorf("failed to compile regexp, err: %s", err)

			continue
		}

		resList := v1.ResourceList{}
		for key, value := range instanceTypeExtension.ExtendedResources {
			resList[v1.ResourceName(key)] = *resource.NewQuantity(value, resource.DecimalSI)
		}

		e.extensions = append(e.extensions, &extension{
			regex:             regex,
			requiredNPLabels:  instanceTypeExtension.RequiredNodePoolLabels,
			extendedResources: resList,
		})
	}
}

func newInstanceTypeExtender(ctx context.Context) *InstanceTypeExtender {
	e := &InstanceTypeExtender{
		solvedMatches: map[string]map[string]*extension{},
		extensions:    []*extension{},
	}

	e.init(ctx)

	return e
}

func GetInstanceTypeExtender(ctx context.Context) *InstanceTypeExtender {
	extOnce.Do(func() {
		extender = newInstanceTypeExtender(ctx)
	})

	return extender
}

func (e *InstanceTypeExtender) AddInstanceTypeExtensions(ctx context.Context, instanceTypeOptions []*cloudprovider.InstanceType, nodePool *v1beta1.NodePool) {
	poolKey := getKey(nodePool)

	if (len(e.solvedMatches[poolKey]) == 0 && len(e.extensions) == 0) ||
		len(instanceTypeOptions) == 0 || nodePool == nil {
		return
	}

	if e.solvedMatches[poolKey] == nil {
		e.solvedMatches[poolKey] = map[string]*extension{}
	}

	for _, instanceType := range instanceTypeOptions {
		e.addInstanceTypeExtensions(poolKey, instanceType, nodePool)
	}
}

func (e *InstanceTypeExtender) addInstanceTypeExtensions(poolKey string, instanceType *cloudprovider.InstanceType, nodePool *v1beta1.NodePool) {
	// prefer already solved instance name matches
	if ext, ok := e.solvedMatches[poolKey][instanceType.Name]; ok {
		if ext != nil {
			addExtendedResources(instanceType, ext)
		}

		return
	}

	// try regexps one at a time, store first match or nil
	var match *extension
	for _, ext := range e.extensions {
		if ext.regex.MatchString(instanceType.Name) {
			if addExtension(instanceType, ext, nodePool) {
				match = ext

				break
			}
		}
	}

	// intentionally store also zero value (nil) for those instance types with no extended resources
	e.solvedMatches[poolKey][instanceType.Name] = match
}

func addExtension(instanceType *cloudprovider.InstanceType, ext *extension, nodePool *v1beta1.NodePool) bool {
	labelsMatch := true
	for labelName, val := range ext.requiredNPLabels {
		if nodePool.Labels[labelName] != val {
			labelsMatch = false
			break
		}
	}

	if labelsMatch {
		addExtendedResources(instanceType, ext)
	}

	return labelsMatch
}

func addExtendedResources(instanceType *cloudprovider.InstanceType, ext *extension) {
	for res, val := range ext.extendedResources {
		instanceType.Capacity[res] = val
	}
}
