/*
Copyright 2020 Cornelius Weig

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

package client

import (
	"context"
	"testing"

	"github.com/berney/rakkess/internal/client/result"
	"github.com/berney/rakkess/internal/constants"
	"github.com/berney/rakkess/internal/options"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/kubernetes/typed/rbac/v1/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	roleNamespace       = "some-ns"
	subjectKind         = "User"
	testClusterRoleName = "some-clusterrole"
	testRoleName        = "some-role"
)

func TestGetSubjectAccess(t *testing.T) {
	tests := []struct {
		name                string
		namespace           string
		resource            string
		apiGroup            string
		clusterRoles        []v1.ClusterRole
		clusterRoleBindings []v1.ClusterRoleBinding
		roles               []v1.Role
		roleBindings        []v1.RoleBinding
		expected            map[result.SubjectRef]sets.String
	}{
		{
			name:                "cluster-role and role matches",
			namespace:           roleNamespace,
			apiGroup:            "apps",
			resource:            "deployments",
			clusterRoles:        clusterRoles("apps", "deployments", "create"),
			clusterRoleBindings: clusterRoleBindings("test-user"),
			roles:               roles("apps", "deployments", "list"),
			roleBindings:        roleBindings(testRoleName, roleName, "test-user"),
			expected: map[result.SubjectRef]sets.String{
				{Name: "test-user", Kind: subjectKind}: sets.NewString("create", "list"),
			},
		},
		{
			name:                "cluster-role and role matches, multiple subjects",
			namespace:           roleNamespace,
			apiGroup:            "apps",
			resource:            "deployments",
			clusterRoles:        clusterRoles("apps", "deployments", "create"),
			clusterRoleBindings: clusterRoleBindings("user1", "user2"),
			roles:               roles("apps", "deployments", "list"),
			roleBindings:        roleBindings(testRoleName, roleName, "user2", "user3"),
			expected: map[result.SubjectRef]sets.String{
				{Name: "user1", Kind: subjectKind}: sets.NewString("create"),
				{Name: "user2", Kind: subjectKind}: sets.NewString("create", "list"),
				{Name: "user3", Kind: subjectKind}: sets.NewString("list"),
			},
		},
		{
			name:                "cluster-role and role matches, global scope",
			namespace:           "", // empty namespace means global scope
			apiGroup:            "apps",
			resource:            "deployments",
			clusterRoles:        clusterRoles("apps", "deployments", "create"),
			clusterRoleBindings: clusterRoleBindings("test-user"),
			roles:               roles("apps", "deployments", "list"),
			roleBindings:        roleBindings(testRoleName, roleName, "test-user"),
			expected: map[result.SubjectRef]sets.String{
				{Name: "test-user", Kind: subjectKind}: sets.NewString("create"),
			},
		},
		{
			name:         "rolebinding to clusterrole",
			namespace:    roleNamespace,
			apiGroup:     "apps",
			resource:     "deployments",
			clusterRoles: clusterRoles("apps", "deployments", "create"),
			roleBindings: roleBindings(testClusterRoleName, clusterRoleName, "test-user"),
			expected: map[result.SubjectRef]sets.String{
				{Name: "test-user", Kind: subjectKind}: sets.NewString("create"),
			},
		},
		{
			name:                "bindings for wrong resource",
			namespace:           roleNamespace,
			apiGroup:            "apps",
			resource:            "deployments",
			clusterRoles:        clusterRoles("", "configmaps", "create"),
			clusterRoleBindings: clusterRoleBindings("test-user"),
			roles:               roles("", "configmaps", "list"),
			roleBindings:        roleBindings(testRoleName, roleName, "test-user"),
			expected:            map[result.SubjectRef]sets.String{},
		},
		{
			name:                "VerbAll role binding",
			namespace:           roleNamespace,
			apiGroup:            "",
			resource:            "configmaps",
			clusterRoles:        clusterRoles("", "configmaps", "create"),
			clusterRoleBindings: clusterRoleBindings("test-user"),
			roles:               roles("", "configmaps", v1.VerbAll),
			roleBindings:        roleBindings(testRoleName, roleName, "test-user"),
			expected: map[result.SubjectRef]sets.String{
				{Name: "test-user", Kind: subjectKind}: sets.NewString(constants.ValidVerbs...),
			},
		},
		{
			name:                "VerbAll clusterrole binding",
			namespace:           roleNamespace,
			apiGroup:            "",
			resource:            "configmaps",
			clusterRoles:        clusterRoles("", "configmaps", v1.VerbAll),
			clusterRoleBindings: clusterRoleBindings("test-user"),
			expected: map[result.SubjectRef]sets.String{
				{Name: "test-user", Kind: subjectKind}: sets.NewString(constants.ValidVerbs...),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()

			fakeRbacClient := &fake.FakeRbacV1{Fake: &k8stesting.Fake{}}
			fakeRbacClient.Fake.AddReactor("list", "roles",
				func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1.RoleList{Items: test.roles}, nil
				})
			fakeRbacClient.Fake.AddReactor("list", "rolebindings",
				func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1.RoleBindingList{Items: test.roleBindings}, nil
				})
			fakeRbacClient.Fake.AddReactor("list", "clusterroles",
				func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1.ClusterRoleList{Items: test.clusterRoles}, nil
				})
			fakeRbacClient.Fake.AddReactor("list", "clusterrolebindings",
				func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1.ClusterRoleBindingList{Items: test.clusterRoleBindings}, nil
				})

			getRbacClient = func(*options.RakkessOptions) (clientv1.RbacV1Interface, error) {
				return fakeRbacClient, nil
			}
			defer func() { getRbacClient = getRbacClientImpl }()

			opts := &options.RakkessOptions{
				ConfigFlags: &genericclioptions.ConfigFlags{
					Namespace: &test.namespace,
				},
			}
			gr := schema.GroupResource{Group: test.apiGroup, Resource: test.resource}
			sa, err := GetSubjectAccess(ctx, opts, gr, "")
			assert.NoError(t, err)
			assert.Equal(t, test.resource, sa.GroupResource.Resource)
			assert.Equal(t, test.apiGroup, sa.GroupResource.Group)
			assert.Equal(t, test.expected, sa.Get())
		})
	}
}

func clusterRoles(apiGroup, resource string, verbs ...string) []v1.ClusterRole {
	return []v1.ClusterRole{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: testClusterRoleName,
			},
			Rules: []v1.PolicyRule{
				{
					APIGroups: []string{apiGroup},
					Verbs:     verbs,
					Resources: []string{resource},
				},
			},
		},
	}
}

func clusterRoleBindings(subjects ...string) []v1.ClusterRoleBinding {
	ss := make([]v1.Subject, 0, len(subjects))
	for _, s := range subjects {
		ss = append(ss, v1.Subject{
			Kind: subjectKind,
			Name: s,
		})
	}
	return []v1.ClusterRoleBinding{
		{
			Subjects: ss,
			RoleRef: v1.RoleRef{
				Name: testClusterRoleName,
				Kind: clusterRoleName,
			},
		},
	}
}

func roles(apiGroup, resource string, verbs ...string) []v1.Role {
	return []v1.Role{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testRoleName,
				Namespace: roleNamespace,
			},
			Rules: []v1.PolicyRule{
				{
					APIGroups: []string{apiGroup},
					Verbs:     verbs,
					Resources: []string{resource},
				},
			},
		},
	}
}

func roleBindings(role, kind string, subjects ...string) []v1.RoleBinding {
	ss := make([]v1.Subject, 0, len(subjects))
	for _, s := range subjects {
		ss = append(ss, v1.Subject{
			Kind: subjectKind,
			Name: s,
		})
	}
	return []v1.RoleBinding{
		{
			Subjects: ss,
			RoleRef: v1.RoleRef{
				Name: role,
				Kind: kind,
			},
		},
	}
}
