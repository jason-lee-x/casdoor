// Copyright 2026 The Casdoor Authors. All Rights Reserved.
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

package object

import (
	"reflect"
	"testing"
)

func getTestWecomDepartments() []*WecomDepartment {
	return []*WecomDepartment{
		{Id: 1, Name: "Example Corporation", ParentId: 0},
		{Id: 10, Name: "Example Division", ParentId: 1},
		{Id: 20, Name: "Platform Engineering", ParentId: 10},
		{Id: 40, Name: "Identity Team", ParentId: 20},
	}
}

func TestWecomUserToOriginalUser(t *testing.T) {
	syncer := &Syncer{
		Organization: "example",
		Type:         "WeCom",
		TableColumns: []*TableColumn{
			{Name: "userid", CasdoorName: "Id", IsKey: true, IsHashed: true},
			{Name: "name", CasdoorName: "RealName", IsHashed: true},
			{Name: "biz_mail", CasdoorName: "Email", IsHashed: true},
			{Name: "mobile", CasdoorName: "Phone", IsHashed: true},
			{Name: "avatar", CasdoorName: "Avatar", IsHashed: true},
			{Name: "position", CasdoorName: "Title", IsHashed: true},
			{Name: "alias", CasdoorName: "DisplayName"},
		},
	}
	provider := &WecomSyncerProvider{Syncer: syncer}
	departmentMap := getWecomDepartmentMap(getTestWecomDepartments())
	wecomUser := &WecomUser{
		UserId:         "alice",
		Name:           "Alice Example",
		Alias:          "Ally",
		Department:     []int{40},
		MainDepartment: 40,
		Email:          "alice@example.com",
		Mobile:         "+12025550123",
		Avatar:         "https://example.com/alice.png",
		Position:       "Software Engineer",
		Gender:         "1",
		Status:         1,
		Enable:         1,
	}

	user := provider.wecomUserToOriginalUser(wecomUser, departmentMap)

	if user.Id != "alice" || user.Name != "alice" {
		t.Fatalf("unexpected user identity: Id=%q Name=%q", user.Id, user.Name)
	}
	if user.RealName != "Alice Example" || user.DisplayName != "Ally" {
		t.Fatalf("unexpected names: RealName=%q DisplayName=%q", user.RealName, user.DisplayName)
	}
	if user.Email != "alice@example.com" || user.Phone != "+12025550123" {
		t.Fatalf("unexpected contacts: Email=%q Phone=%q", user.Email, user.Phone)
	}
	if user.Affiliation != "Example Division/Platform Engineering/Identity Team" {
		t.Fatalf("unexpected affiliation: %q", user.Affiliation)
	}
	if !reflect.DeepEqual(user.Groups, []string{"example/40"}) {
		t.Fatalf("unexpected groups: %#v", user.Groups)
	}
}

func TestWecomDepartmentToOriginalGroup(t *testing.T) {
	provider := &WecomSyncerProvider{Syncer: &Syncer{Organization: "example"}}
	departmentMap := getWecomDepartmentMap(getTestWecomDepartments())

	if group := provider.wecomDepartmentToOriginalGroup(departmentMap[1], departmentMap); group != nil {
		t.Fatalf("root department should not be duplicated as a Casdoor group: %#v", group)
	}

	topGroup := provider.wecomDepartmentToOriginalGroup(departmentMap[10], departmentMap)
	if topGroup.Name != "10" || topGroup.ParentId != "example" || topGroup.Type != "Physical" {
		t.Fatalf("unexpected top group: %#v", topGroup)
	}

	childGroup := provider.wecomDepartmentToOriginalGroup(departmentMap[40], departmentMap)
	if childGroup.Name != "40" || childGroup.ParentId != "20" || childGroup.DisplayName != "Identity Team" {
		t.Fatalf("unexpected child group: %#v", childGroup)
	}
}

func TestWecomSensitiveFieldFallbacks(t *testing.T) {
	provider := &WecomSyncerProvider{Syncer: &Syncer{}}
	wecomUser := &WecomUser{
		BizMail:     "fallback@example.com",
		ThumbAvatar: "https://example.com/thumb.png",
	}

	if got := provider.getWecomUserFieldValue(wecomUser, "email", ""); got != "fallback@example.com" {
		t.Fatalf("unexpected email fallback: %q", got)
	}
	if got := provider.getWecomUserFieldValue(wecomUser, "avatar", ""); got != "https://example.com/thumb.png" {
		t.Fatalf("unexpected avatar fallback: %q", got)
	}
}
