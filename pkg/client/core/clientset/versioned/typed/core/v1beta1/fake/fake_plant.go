/*
Copyright (c) SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"

	v1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// FakePlants implements PlantInterface
type FakePlants struct {
	Fake *FakeCoreV1beta1
	ns   string
}

var plantsResource = schema.GroupVersionResource{Group: "core.gardener.cloud", Version: "v1beta1", Resource: "plants"}

var plantsKind = schema.GroupVersionKind{Group: "core.gardener.cloud", Version: "v1beta1", Kind: "Plant"}

// Get takes name of the plant, and returns the corresponding plant object, and an error if there is any.
func (c *FakePlants) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.Plant, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(plantsResource, c.ns, name), &v1beta1.Plant{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Plant), err
}

// List takes label and field selectors, and returns the list of Plants that match those selectors.
func (c *FakePlants) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.PlantList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(plantsResource, plantsKind, c.ns, opts), &v1beta1.PlantList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.PlantList{ListMeta: obj.(*v1beta1.PlantList).ListMeta}
	for _, item := range obj.(*v1beta1.PlantList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested plants.
func (c *FakePlants) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(plantsResource, c.ns, opts))

}

// Create takes the representation of a plant and creates it.  Returns the server's representation of the plant, and an error, if there is any.
func (c *FakePlants) Create(ctx context.Context, plant *v1beta1.Plant, opts v1.CreateOptions) (result *v1beta1.Plant, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(plantsResource, c.ns, plant), &v1beta1.Plant{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Plant), err
}

// Update takes the representation of a plant and updates it. Returns the server's representation of the plant, and an error, if there is any.
func (c *FakePlants) Update(ctx context.Context, plant *v1beta1.Plant, opts v1.UpdateOptions) (result *v1beta1.Plant, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(plantsResource, c.ns, plant), &v1beta1.Plant{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Plant), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakePlants) UpdateStatus(ctx context.Context, plant *v1beta1.Plant, opts v1.UpdateOptions) (*v1beta1.Plant, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(plantsResource, "status", c.ns, plant), &v1beta1.Plant{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Plant), err
}

// Delete takes name of the plant and deletes it. Returns an error if one occurs.
func (c *FakePlants) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(plantsResource, c.ns, name, opts), &v1beta1.Plant{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePlants) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(plantsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1beta1.PlantList{})
	return err
}

// Patch applies the patch and returns the patched plant.
func (c *FakePlants) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.Plant, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(plantsResource, c.ns, name, pt, data, subresources...), &v1beta1.Plant{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Plant), err
}
