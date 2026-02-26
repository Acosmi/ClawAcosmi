package line

// TS 对照: src/line/rich-menu.ts (466L)
// LINE Rich Menu 管理 — 创建、上传图片、设置默认、绑定用户等操作

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ---------- Rich Menu API 扩展 ----------
// 注: reply_chunks.go 中已有 RichMenu / RichMenuSize / RichMenuArea /
// RichMenuBounds 及 CreateRichMenu / SetDefaultRichMenu / LinkRichMenuToUser。
// 本文件补充 TS 端缺失的 API 方法，避免重复定义。

const richMenuBaseURL = "https://api.line.me/v2/bot"

// UploadRichMenuImage 上传 Rich Menu 图片。
// TS: uploadRichMenuImage()
func (c *Client) UploadRichMenuImage(ctx context.Context, richMenuID, imagePath string) error {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("line: read rich menu image: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(imagePath))
	contentType := "image/jpeg"
	if ext == ".png" {
		contentType = "image/png"
	}

	url := fmt.Sprintf("https://api-data.line.me/v2/bot/richmenu/%s/content", richMenuID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.ChannelToken)
	req.Header.Set("Content-Type", contentType)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("line: upload rich menu image HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// CancelDefaultRichMenu 取消默认 Rich Menu。
// TS: cancelDefaultRichMenu()
func (c *Client) CancelDefaultRichMenu(ctx context.Context) error {
	url := richMenuBaseURL + "/user/all/richmenu"
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.ChannelToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("line: cancel default rich menu HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetDefaultRichMenuID 获取默认 Rich Menu ID。
// TS: getDefaultRichMenuId()
func (c *Client) GetDefaultRichMenuID(ctx context.Context) (string, error) {
	url := richMenuBaseURL + "/user/all/richmenu"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.ChannelToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", nil
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("line: get default rich menu HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		RichMenuID string `json:"richMenuId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.RichMenuID, nil
}

// GetRichMenuList 获取所有 Rich Menu 列表。
// TS: getRichMenuList()
func (c *Client) GetRichMenuList(ctx context.Context) ([]RichMenu, error) {
	url := richMenuBaseURL + "/richmenu/list"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.ChannelToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("line: get rich menu list HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Richmenus []RichMenu `json:"richmenus"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Richmenus, nil
}

// GetRichMenu 获取指定 Rich Menu。
// TS: getRichMenu()
func (c *Client) GetRichMenu(ctx context.Context, richMenuID string) (*RichMenu, error) {
	url := richMenuBaseURL + "/richmenu/" + richMenuID
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.ChannelToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, nil
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("line: get rich menu HTTP %d: %s", resp.StatusCode, string(body))
	}

	var menu RichMenu
	if err := json.NewDecoder(resp.Body).Decode(&menu); err != nil {
		return nil, err
	}
	return &menu, nil
}

// DeleteRichMenu 删除 Rich Menu。
// TS: deleteRichMenu()
func (c *Client) DeleteRichMenu(ctx context.Context, richMenuID string) error {
	url := richMenuBaseURL + "/richmenu/" + richMenuID
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.ChannelToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("line: delete rich menu HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// LinkRichMenuToUsers 批量绑定 Rich Menu（每批最多 500 用户）。
// TS: linkRichMenuToUsers()
func (c *Client) LinkRichMenuToUsers(ctx context.Context, userIDs []string, richMenuID string) error {
	for i := 0; i < len(userIDs); i += 500 {
		end := i + 500
		if end > len(userIDs) {
			end = len(userIDs)
		}
		batch := userIDs[i:end]
		body := map[string]interface{}{
			"richMenuId": richMenuID,
			"userIds":    batch,
		}
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		if err := c.postRaw(ctx, "/richmenu/bulk/link", data); err != nil {
			return err
		}
	}
	return nil
}

// UnlinkRichMenuFromUser 解绑用户的 Rich Menu。
// TS: unlinkRichMenuFromUser()
func (c *Client) UnlinkRichMenuFromUser(ctx context.Context, userID string) error {
	return c.post(ctx, "/user/"+userID+"/richmenu", nil)
}

// UnlinkRichMenuFromUsers 批量解绑 Rich Menu（每批最多 500 用户）。
// TS: unlinkRichMenuFromUsers()
func (c *Client) UnlinkRichMenuFromUsers(ctx context.Context, userIDs []string) error {
	for i := 0; i < len(userIDs); i += 500 {
		end := i + 500
		if end > len(userIDs) {
			end = len(userIDs)
		}
		batch := userIDs[i:end]
		body := map[string]interface{}{"userIds": batch}
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		if err := c.postRaw(ctx, "/richmenu/bulk/unlink", data); err != nil {
			return err
		}
	}
	return nil
}

// GetRichMenuIDOfUser 获取用户绑定的 Rich Menu ID。
// TS: getRichMenuIdOfUser()
func (c *Client) GetRichMenuIDOfUser(ctx context.Context, userID string) (string, error) {
	url := c.BaseURL + "/user/" + userID + "/richmenu"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.ChannelToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", nil
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("line: get rich menu of user HTTP %d", resp.StatusCode)
	}

	var result struct {
		RichMenuID string `json:"richMenuId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.RichMenuID, nil
}

// CreateRichMenuAlias 创建 Rich Menu 别名。
// TS: createRichMenuAlias()
func (c *Client) CreateRichMenuAlias(ctx context.Context, richMenuID, aliasID string) error {
	body := map[string]string{
		"richMenuId":      richMenuID,
		"richMenuAliasId": aliasID,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return c.postRaw(ctx, "/richmenu/alias", data)
}

// DeleteRichMenuAlias 删除 Rich Menu 别名。
// TS: deleteRichMenuAlias()
func (c *Client) DeleteRichMenuAlias(ctx context.Context, aliasID string) error {
	url := c.BaseURL + "/richmenu/alias/" + aliasID
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.ChannelToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("line: delete rich menu alias HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// postRaw 发送原始 JSON body 的 POST 请求。
func (c *Client) postRaw(ctx context.Context, path string, data []byte) error {
	url := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.ChannelToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LINE API error %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// ---------- Layout Helpers (TS: createGridLayout etc.) ----------

// RichMenuGridArea Rich Menu 格子区域。
type RichMenuGridArea struct {
	Bounds RichMenuBounds `json:"bounds"`
	Action FlexAction     `json:"action"`
}

// CreateRichMenuGridLayout 创建标准 2x3 格子布局（6 个区域）。
// TS: createGridLayout()
func CreateRichMenuGridLayout(height int, actions [6]FlexAction) []RichMenuArea {
	colWidth := 2500 / 3
	rowHeight := height / 2

	return []RichMenuArea{
		// 上排
		{Bounds: RichMenuBounds{X: 0, Y: 0, Width: colWidth, Height: rowHeight}, Action: actions[0]},
		{Bounds: RichMenuBounds{X: colWidth, Y: 0, Width: colWidth, Height: rowHeight}, Action: actions[1]},
		{Bounds: RichMenuBounds{X: colWidth * 2, Y: 0, Width: colWidth, Height: rowHeight}, Action: actions[2]},
		// 下排
		{Bounds: RichMenuBounds{X: 0, Y: rowHeight, Width: colWidth, Height: rowHeight}, Action: actions[3]},
		{Bounds: RichMenuBounds{X: colWidth, Y: rowHeight, Width: colWidth, Height: rowHeight}, Action: actions[4]},
		{Bounds: RichMenuBounds{X: colWidth * 2, Y: rowHeight, Width: colWidth, Height: rowHeight}, Action: actions[5]},
	}
}

// MessageAction 创建消息动作（点击后发送文本）。
// TS: messageAction()
func MessageAction(label, text string) FlexAction {
	if len(label) > 20 {
		label = label[:20]
	}
	if text == "" {
		text = label
	}
	return FlexAction{Type: "message", Label: label, Text: text}
}

// URIAction 创建 URI 动作（点击后打开 URL）。
// TS: uriAction()
func URIAction(label, uri string) FlexAction {
	if len(label) > 20 {
		label = label[:20]
	}
	return FlexAction{Type: "uri", Label: label, URI: uri}
}

// PostbackAction 创建 postback 动作。
// TS: postbackAction()
func PostbackAction(label, data, displayText string) FlexAction {
	if len(label) > 20 {
		label = label[:20]
	}
	if len(data) > 300 {
		data = data[:300]
	}
	a := FlexAction{Type: "postback", Label: label, Data: data}
	if displayText != "" {
		a.Text = displayText
	}
	return a
}

// CreateDefaultMenuConfig 创建默认菜单配置（help/status/settings 等）。
// TS: createDefaultMenuConfig()
func CreateDefaultMenuConfig() RichMenu {
	return RichMenu{
		Size:        RichMenuSize{Width: 2500, Height: 843},
		Selected:    false,
		Name:        "Default Menu",
		ChatBarText: "Menu",
		Areas: CreateRichMenuGridLayout(843, [6]FlexAction{
			MessageAction("Help", "/help"),
			MessageAction("Status", "/status"),
			MessageAction("Settings", "/settings"),
			MessageAction("About", "/about"),
			MessageAction("Feedback", "/feedback"),
			MessageAction("Contact", "/contact"),
		}),
	}
}
