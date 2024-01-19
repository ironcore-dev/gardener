// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package genericactuator_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry"
	"github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator"
	extensionsmockgenericactuator "github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator/mock"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockmanager "github.com/gardener/gardener/pkg/mock/controller-runtime/manager"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

const (
	providerSecretName      = "backupprovider"
	providerSecretNamespace = "garden"
	shootTechnicalID        = "shoot--foo--bar"
	shootUID                = "asd234-asd-34"
	bucketName              = "test-bucket"
)

var _ = Describe("Actuator", func() {
	var (
		ctx = context.Background()
		log = logf.Log.WithName("test")

		ctrl *gomock.Controller
		mgr  *mockmanager.MockManager

		backupEntry              *extensionsv1alpha1.BackupEntry
		backupProviderSecretData map[string][]byte
		backupEntrySecret        *corev1.Secret
		etcdBackupSecretData     map[string][]byte
		etcdBackupSecretKey      client.ObjectKey
		etcdBackupSecret         *corev1.Secret
		seedNamespace            *corev1.Namespace

		fakeClient client.Client
		a          backupentry.Actuator
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		backupEntry = &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootTechnicalID + "--" + shootUID,
			},
			Spec: extensionsv1alpha1.BackupEntrySpec{
				BucketName: bucketName,
				SecretRef: corev1.SecretReference{
					Name:      providerSecretName,
					Namespace: providerSecretNamespace,
				},
			},
		}
		backupProviderSecretData = map[string][]byte{
			"foo":        []byte("bar"),
			"bucketName": []byte(bucketName),
		}
		backupEntrySecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      providerSecretName,
				Namespace: providerSecretNamespace,
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}
		etcdBackupSecretData = map[string][]byte{
			"bucketName": []byte(bucketName),
			"foo":        []byte("bar"),
		}
		etcdBackupSecretKey = client.ObjectKey{Namespace: shootTechnicalID, Name: v1beta1constants.BackupSecretName}
		etcdBackupSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.BackupSecretName,
				Namespace: shootTechnicalID,
			},
			Data: etcdBackupSecretData,
		}
		seedNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootTechnicalID,
			},
		}

		mgr = mockmanager.NewMockManager(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("#Reconcile", func() {
		var backupEntryDelegate *extensionsmockgenericactuator.MockBackupEntryDelegate

		BeforeEach(func() {
			backupEntryDelegate = extensionsmockgenericactuator.NewMockBackupEntryDelegate(ctrl)
		})

		Context("seed namespace exist", func() {
			It("should create secrets", func() {
				fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(seedNamespace, backupEntrySecret).Build()
				mgr.EXPECT().GetClient().Return(fakeClient)
				backupEntryDelegate.EXPECT().GetETCDSecretData(ctx, gomock.AssignableToTypeOf(logr.Logger{}), backupEntry, backupProviderSecretData).Return(etcdBackupSecretData, nil)

				a = genericactuator.NewActuator(mgr, backupEntryDelegate)
				Expect(a.Reconcile(ctx, log, backupEntry)).To(Succeed())

				actual := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, etcdBackupSecretKey, actual)).To(Succeed())
				etcdBackupSecret.Annotations = map[string]string{
					genericactuator.AnnotationKeyCreatedByBackupEntry: backupEntry.Name,
				}
				etcdBackupSecret.ResourceVersion = "1"
				Expect(actual).To(Equal(etcdBackupSecret))
			})
		})

		Context("seed namespace does not exist", func() {
			It("should not create secrets", func() {
				fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(backupEntrySecret).Build()
				mgr.EXPECT().GetClient().Return(fakeClient)

				a = genericactuator.NewActuator(mgr, backupEntryDelegate)
				Expect(a.Reconcile(ctx, log, backupEntry)).To(Succeed())

				Expect(fakeClient.Get(ctx, etcdBackupSecretKey, &corev1.Secret{})).To(BeNotFoundError())
			})
		})
	})

	Context("#Delete", func() {
		var backupEntryDelegate *extensionsmockgenericactuator.MockBackupEntryDelegate

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(seedNamespace, backupEntrySecret).Build()
			mgr.EXPECT().GetClient().Return(fakeClient)

			backupEntryDelegate = extensionsmockgenericactuator.NewMockBackupEntryDelegate(ctrl)
			backupEntryDelegate.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(logr.Logger{}), backupEntry).Return(nil)
		})

		It("should not delete secret if has annotation with different BackupEntry name", func() {
			etcdBackupSecret.Annotations = map[string]string{
				genericactuator.AnnotationKeyCreatedByBackupEntry: "foo",
			}
			Expect(fakeClient.Create(ctx, etcdBackupSecret)).To(Succeed())

			a = genericactuator.NewActuator(mgr, backupEntryDelegate)
			Expect(a.Delete(ctx, log, backupEntry)).To(Succeed())

			actual := &corev1.Secret{}
			Expect(fakeClient.Get(ctx, etcdBackupSecretKey, actual)).To(Succeed())
			etcdBackupSecret.ResourceVersion = "1"
			Expect(actual).To(Equal(etcdBackupSecret))
		})

		It("should delete secret if it has annotation with same BackupEntry name", func() {
			etcdBackupSecret.Annotations = map[string]string{
				genericactuator.AnnotationKeyCreatedByBackupEntry: backupEntry.Name,
			}
			Expect(fakeClient.Create(ctx, etcdBackupSecret)).To(Succeed())

			a = genericactuator.NewActuator(mgr, backupEntryDelegate)
			Expect(a.Delete(ctx, log, backupEntry)).To(Succeed())

			Expect(fakeClient.Get(ctx, etcdBackupSecretKey, &corev1.Secret{})).To(BeNotFoundError())
		})

		It("should delete secret if it has no annotations", func() {
			Expect(fakeClient.Create(ctx, etcdBackupSecret)).To(Succeed())

			a = genericactuator.NewActuator(mgr, backupEntryDelegate)
			Expect(a.Delete(ctx, log, backupEntry)).To(Succeed())

			Expect(fakeClient.Get(ctx, etcdBackupSecretKey, &corev1.Secret{})).To(BeNotFoundError())
		})
	})
})
