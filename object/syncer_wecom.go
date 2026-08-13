// Copyright 2025 The Casdoor Authors. All Rights Reserved.
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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/casdoor/casdoor/util"
)

// WecomSyncerProvider implements SyncerProvider for WeCom (WeChat Work) API-based syncers
type WecomSyncerProvider struct {
	Syncer *Syncer
}

// InitAdapter initializes the WeCom syncer (no database adapter needed)
func (p *WecomSyncerProvider) InitAdapter() error {
	// WeCom syncer doesn't need database adapter
	return nil
}

// GetOriginalUsers retrieves all users from WeCom API
func (p *WecomSyncerProvider) GetOriginalUsers() ([]*OriginalUser, error) {
	return p.getWecomUsers()
}

// AddUser adds a new user to WeCom (not supported for read-only API)
func (p *WecomSyncerProvider) AddUser(user *OriginalUser) (bool, error) {
	// WeCom syncer is typically read-only
	return false, fmt.Errorf("adding users to WeCom is not supported")
}

// UpdateUser updates an existing user in WeCom (not supported for read-only API)
func (p *WecomSyncerProvider) UpdateUser(user *OriginalUser) (bool, error) {
	// WeCom syncer is typically read-only
	return false, fmt.Errorf("updating users in WeCom is not supported")
}

// TestConnection tests the WeCom API connection
func (p *WecomSyncerProvider) TestConnection() error {
	_, err := p.getWecomAccessToken()
	return err
}

// Close closes any open connections (no-op for WeCom API-based syncer)
func (p *WecomSyncerProvider) Close() error {
	// WeCom syncer doesn't maintain persistent connections
	return nil
}

type WecomAccessTokenResp struct {
	Errcode     int    `json:"errcode"`
	Errmsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type WecomUser struct {
	UserId         string `json:"userid"`
	Name           string `json:"name"`
	Alias          string `json:"alias"`
	Department     []int  `json:"department"`
	MainDepartment int    `json:"main_department"`
	Position       string `json:"position"`
	Mobile         string `json:"mobile"`
	Gender         string `json:"gender"`
	Email          string `json:"email"`
	BizMail        string `json:"biz_mail"`
	Avatar         string `json:"avatar"`
	ThumbAvatar    string `json:"thumb_avatar"`
	Status         int    `json:"status"`
	Enable         int    `json:"enable"`
}

type WecomUserListResp struct {
	Errcode  int          `json:"errcode"`
	Errmsg   string       `json:"errmsg"`
	Userlist []*WecomUser `json:"userlist"`
}

type WecomDeptListResp struct {
	Errcode    int                `json:"errcode"`
	Errmsg     string             `json:"errmsg"`
	Department []*WecomDepartment `json:"department"`
}

type WecomDepartment struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	ParentId int    `json:"parentid"`
}

type WecomUserResp struct {
	Errcode int    `json:"errcode"`
	Errmsg  string `json:"errmsg"`
	WecomUser
}

// getWecomAccessToken gets access token from WeCom API
func (p *WecomSyncerProvider) getWecomAccessToken() (string, error) {
	apiUrl := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		url.QueryEscape(p.Syncer.User), url.QueryEscape(p.Syncer.Password))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiUrl, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp WecomAccessTokenResp
	err = json.Unmarshal(data, &tokenResp)
	if err != nil {
		return "", err
	}

	if tokenResp.Errcode != 0 {
		return "", fmt.Errorf("failed to get access token: errcode=%d, errmsg=%s",
			tokenResp.Errcode, tokenResp.Errmsg)
	}

	return tokenResp.AccessToken, nil
}

// getWecomDepartments gets all departments from WeCom API
func (p *WecomSyncerProvider) getWecomDepartments(accessToken string) ([]*WecomDepartment, error) {
	apiUrl := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/department/list?access_token=%s",
		url.QueryEscape(accessToken))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiUrl, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var deptResp WecomDeptListResp
	err = json.Unmarshal(data, &deptResp)
	if err != nil {
		return nil, err
	}

	if deptResp.Errcode != 0 {
		return nil, fmt.Errorf("failed to get departments: errcode=%d, errmsg=%s",
			deptResp.Errcode, deptResp.Errmsg)
	}

	return deptResp.Department, nil
}

// getWecomUsersFromDept gets users from a specific department
func (p *WecomSyncerProvider) getWecomUsersFromDept(accessToken string, deptId int) ([]*WecomUser, error) {
	apiUrl := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/user/list?access_token=%s&department_id=%d",
		url.QueryEscape(accessToken), deptId)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiUrl, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userResp WecomUserListResp
	err = json.Unmarshal(data, &userResp)
	if err != nil {
		return nil, err
	}

	if userResp.Errcode != 0 {
		return nil, fmt.Errorf("failed to get users from dept %d: errcode=%d, errmsg=%s",
			deptId, userResp.Errcode, userResp.Errmsg)
	}

	return userResp.Userlist, nil
}

// getWecomUsers gets all users from WeCom API
func (p *WecomSyncerProvider) getWecomUsers() ([]*OriginalUser, error) {
	// Get access token
	accessToken, err := p.getWecomAccessToken()
	if err != nil {
		return nil, err
	}

	// Get all departments
	departments, err := p.getWecomDepartments(accessToken)
	if err != nil {
		return nil, err
	}
	departmentMap := getWecomDepartmentMap(departments)

	// Get users from all departments (deduplicate by userid)
	userMap := make(map[string]*WecomUser)
	for _, department := range departments {
		users, err := p.getWecomUsersFromDept(accessToken, department.Id)
		if err != nil {
			return nil, err
		}

		for _, user := range users {
			// Deduplicate users by userid
			if _, exists := userMap[user.UserId]; !exists {
				userMap[user.UserId] = user
			}
		}
	}

	// Convert WeCom users to Casdoor OriginalUser
	originalUsers := []*OriginalUser{}
	for _, wecomUser := range userMap {
		originalUser := p.wecomUserToOriginalUser(wecomUser, departmentMap)
		originalUsers = append(originalUsers, originalUser)
	}

	return originalUsers, nil
}

func getWecomDepartmentMap(departments []*WecomDepartment) map[int]*WecomDepartment {
	departmentMap := make(map[int]*WecomDepartment, len(departments))
	for _, department := range departments {
		departmentMap[department.Id] = department
	}
	return departmentMap
}

func getWecomDepartmentPath(departmentId int, departmentMap map[int]*WecomDepartment) string {
	names := []string{}
	visited := map[int]bool{}
	for departmentId != 0 && !visited[departmentId] {
		visited[departmentId] = true
		department, ok := departmentMap[departmentId]
		if !ok {
			break
		}
		// parentid=0 的节点是企业根部门，Casdoor 已用 Organization 表示这一层。
		if department.ParentId != 0 {
			names = append(names, department.Name)
		}
		departmentId = department.ParentId
	}

	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	return strings.Join(names, "/")
}

func getWecomGender(gender string) string {
	switch gender {
	case "1":
		return "Male"
	case "2":
		return "Female"
	default:
		return ""
	}
}

func getFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (p *WecomSyncerProvider) getWecomUserFieldValue(wecomUser *WecomUser, fieldName string, affiliation string) string {
	switch fieldName {
	case "userid":
		return wecomUser.UserId
	case "name":
		return wecomUser.Name
	case "alias":
		return wecomUser.Alias
	case "email":
		return getFirstNonEmpty(wecomUser.Email, wecomUser.BizMail)
	case "biz_mail":
		return getFirstNonEmpty(wecomUser.BizMail, wecomUser.Email)
	case "mobile":
		return wecomUser.Mobile
	case "avatar", "thumb_avatar":
		return getFirstNonEmpty(wecomUser.Avatar, wecomUser.ThumbAvatar)
	case "position":
		return wecomUser.Position
	case "gender":
		return getWecomGender(wecomUser.Gender)
	case "department", "main_department":
		return affiliation
	default:
		return ""
	}
}

// wecomUserToOriginalUser converts WeCom user to Casdoor OriginalUser
func (p *WecomSyncerProvider) wecomUserToOriginalUser(wecomUser *WecomUser, departmentMap map[int]*WecomDepartment) *OriginalUser {
	mainDepartment := wecomUser.MainDepartment
	if mainDepartment == 0 && len(wecomUser.Department) > 0 {
		mainDepartment = wecomUser.Department[0]
	}
	affiliation := getWecomDepartmentPath(mainDepartment, departmentMap)

	user := &OriginalUser{
		Address:     []string{},
		Properties:  map[string]string{},
		Groups:      []string{},
		Affiliation: affiliation,
		Wecom:       wecomUser.UserId,
	}

	if len(p.Syncer.TableColumns) > 0 {
		for _, tableColumn := range p.Syncer.TableColumns {
			value := p.getWecomUserFieldValue(wecomUser, tableColumn.Name, affiliation)
			p.Syncer.setUserByKeyValue(user, tableColumn.CasdoorName, value)
		}
	} else {
		user.Id = wecomUser.UserId
		user.Name = wecomUser.UserId
		user.RealName = wecomUser.Name
		user.DisplayName = getFirstNonEmpty(wecomUser.Alias, wecomUser.Name)
		user.Email = getFirstNonEmpty(wecomUser.BizMail, wecomUser.Email)
		user.Phone = wecomUser.Mobile
		user.Avatar = getFirstNonEmpty(wecomUser.Avatar, wecomUser.ThumbAvatar)
		user.Title = wecomUser.Position
		user.Gender = getWecomGender(wecomUser.Gender)
	}
	if user.Id == "" {
		user.Id = wecomUser.UserId
	}
	if user.Name == "" {
		user.Name = wecomUser.UserId
	}
	if user.RealName == "" {
		user.RealName = wecomUser.Name
	}
	if user.DisplayName == "" {
		user.DisplayName = getFirstNonEmpty(wecomUser.Alias, wecomUser.Name)
	}
	for _, departmentId := range wecomUser.Department {
		department, ok := departmentMap[departmentId]
		if !ok || department.ParentId == 0 {
			continue
		}
		user.Groups = append(user.Groups, fmt.Sprintf("%s/%d", p.Syncer.Organization, departmentId))
	}
	sort.Strings(user.Groups)

	// Set IsForbidden based on status
	// status: 1=activated, 2=disabled, 4=not activated, 5=quit
	// enable: 1=enabled, 0=disabled
	if wecomUser.Status == 2 || wecomUser.Status == 4 || wecomUser.Status == 5 || wecomUser.Enable == 0 {
		user.IsForbidden = true
	} else {
		user.IsForbidden = false
	}

	// Set CreatedTime to current time if not set
	if user.CreatedTime == "" {
		user.CreatedTime = util.GetCurrentTime()
	}

	return user
}

func (p *WecomSyncerProvider) wecomDepartmentToOriginalGroup(department *WecomDepartment, departmentMap map[int]*WecomDepartment) *OriginalGroup {
	if department.ParentId == 0 {
		return nil
	}

	parentId := p.Syncer.Organization
	if parent, ok := departmentMap[department.ParentId]; ok && parent.ParentId != 0 {
		parentId = strconv.Itoa(parent.Id)
	}
	return &OriginalGroup{
		Id:          strconv.Itoa(department.Id),
		Name:        strconv.Itoa(department.Id),
		DisplayName: department.Name,
		Type:        "Physical",
		ParentId:    parentId,
	}
}

// GetOriginalGroups retrieves all department groups from WeCom
func (p *WecomSyncerProvider) GetOriginalGroups() ([]*OriginalGroup, error) {
	accessToken, err := p.getWecomAccessToken()
	if err != nil {
		return nil, err
	}
	departments, err := p.getWecomDepartments(accessToken)
	if err != nil {
		return nil, err
	}
	departmentMap := getWecomDepartmentMap(departments)
	originalGroups := make([]*OriginalGroup, 0, len(departments))
	for _, department := range departments {
		originalGroup := p.wecomDepartmentToOriginalGroup(department, departmentMap)
		if originalGroup != nil {
			originalGroups = append(originalGroups, originalGroup)
		}
	}
	return originalGroups, nil
}

func (p *WecomSyncerProvider) getWecomUser(accessToken string, userId string) (*WecomUser, error) {
	apiUrl := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/user/get?access_token=%s&userid=%s",
		url.QueryEscape(accessToken), url.QueryEscape(userId))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", apiUrl, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var userResp WecomUserResp
	if err = json.Unmarshal(data, &userResp); err != nil {
		return nil, err
	}
	if userResp.Errcode != 0 {
		return nil, fmt.Errorf("failed to get user %s: errcode=%d, errmsg=%s", userId, userResp.Errcode, userResp.Errmsg)
	}
	return &userResp.WecomUser, nil
}

// GetOriginalUserGroups retrieves the department group IDs that a user belongs to
func (p *WecomSyncerProvider) GetOriginalUserGroups(userId string) ([]string, error) {
	accessToken, err := p.getWecomAccessToken()
	if err != nil {
		return nil, err
	}
	wecomUser, err := p.getWecomUser(accessToken, userId)
	if err != nil {
		return nil, err
	}
	departments, err := p.getWecomDepartments(accessToken)
	if err != nil {
		return nil, err
	}
	departmentMap := getWecomDepartmentMap(departments)
	groups := make([]string, 0, len(wecomUser.Department))
	for _, departmentId := range wecomUser.Department {
		department, ok := departmentMap[departmentId]
		if !ok || department.ParentId == 0 {
			continue
		}
		groups = append(groups, fmt.Sprintf("%s/%d", p.Syncer.Organization, departmentId))
	}
	sort.Strings(groups)
	return groups, nil
}
