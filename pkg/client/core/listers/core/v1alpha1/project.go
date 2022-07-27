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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	v1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
)

// ProjectLister helps list Projects.
// All objects returned here must be treated as read-only.
type ProjectLister interface {
	// List lists all Projects in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.Project, err error)
	// Get retrieves the Project from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.Project, error)
	ProjectListerExpansion
}

// projectLister implements the ProjectLister interface.
type projectLister struct {
	indexer cache.Indexer
}

// NewProjectLister returns a new ProjectLister.
func NewProjectLister(indexer cache.Indexer) ProjectLister {
	return &projectLister{indexer: indexer}
}

// List lists all Projects in the indexer.
func (s *projectLister) List(selector labels.Selector) (ret []*v1alpha1.Project, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.Project))
	})
	return ret, err
}

// Get retrieves the Project from the index for a given name.
func (s *projectLister) Get(name string) (*v1alpha1.Project, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("project"), name)
	}
	return obj.(*v1alpha1.Project), nil
}
