// Copyright 2019 Google LLC
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
//
// Copyright 2019 Google LLC. This software is provided as-is,
// without warranty or representation for any use or purpose.
//

package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	appconfigmgrv1alpha1 "github.com/GoogleCloudPlatform/anthos-appconfig/appconfigmgrv2/api/v1alpha1"
)

const (
	VAULT_CONFIGMAP_NAME = "vault"
	VAULT_CA_SECRET_NAME = "vault-ca"
	VAULT_ACM_ROLE       = "acm-vault"
	TODO_FIND_NAMESPACE  = "appconfigmgrv2-system"
)

func (r *AppEnvConfigTemplateV2Reconciler) reconcileVault(
	ctx context.Context,
	meta metav1.ObjectMeta,
	vaultInfo *appconfigmgrv1alpha1.AppEnvConfigTemplateGCPAccessVaultInfo,
) error {
	log.Info("Starting Vault reconcile")
	defer log.Info("Vault reconcile complete")

	// defaults if unspecified
	if vaultInfo.GCPPath == "" {
		vaultInfo.GCPPath = "gcp"
	}
	if vaultInfo.K8SPath == "" {
		vaultInfo.K8SPath = "kubernetes"
	}

	cm := &corev1.ConfigMap{}
	name := types.NamespacedName{
		Name:      VAULT_CONFIGMAP_NAME,
		Namespace: TODO_FIND_NAMESPACE,
	}
	if err := r.Client.Get(ctx, name, cm); err != nil {
		return fmt.Errorf("finding %s ConfigMap: %s", VAULT_CONFIGMAP_NAME, err)
	}

	vaultClient := newVaultClient(cm.Data["vault-addr"])
	if err := vaultClient.login(vaultInfo.K8SPath, cm.Data["serviceaccount-jwt"]); err != nil {
		return fmt.Errorf("instantiating vault client: %s", err)
	}

	log.Info("Reconciling", "vault", "policy")

	policyName := fmt.Sprintf("%s-%s-%s", vaultInfo.GCPPath, meta.Namespace, meta.Name)
	policy := vaultPolicy{
		Path:         fmt.Sprintf("%s/key/%s", vaultInfo.GCPPath, vaultInfo.RoleSet),
		Capabilities: []string{"read"},
	}

	if err := vaultClient.setPolicy(policyName, policy); err != nil {
		return err
	}

	log.Info("Reconciling", "vault", "auth roles")

	k8sroleName := fmt.Sprintf("%s-%s", meta.Namespace, meta.Name)
	k8srole := createRoleRequest{
		BoundServiceAccountNames:      meta.Name,
		BoundServiceAccountNamespaces: meta.Namespace,
		MaxTTL:                        3600,
		Policies:                      []string{policyName},
	}

	if err := vaultClient.setK8SRole(vaultInfo.K8SPath, k8sroleName, k8srole); err != nil {
		return err
	}

	return nil
}