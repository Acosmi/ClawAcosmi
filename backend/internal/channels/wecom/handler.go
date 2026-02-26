package wecom

// handler.go — 企业微信消息回调处理
// 使用标准库实现 AES 解密 + 签名验证
// 参考: https://developer.work.weixin.qq.com/document/path/90238

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
)

// CallbackHandler 企业微信 HTTP 回调处理器
type CallbackHandler struct {
	client    *WeComClient
	onMessage func(msgType, content, fromUser string)
}

// CallbackHandlerConfig 回调处理器配置
type CallbackHandlerConfig struct {
	Client    *WeComClient
	OnMessage func(msgType, content, fromUser string)
}

// NewCallbackHandler 创建回调处理器
func NewCallbackHandler(cfg CallbackHandlerConfig) *CallbackHandler {
	return &CallbackHandler{
		client:    cfg.Client,
		onMessage: cfg.OnMessage,
	}
}

// VerifyURL 验证企业微信回调 URL（GET 请求处理）
// 返回解密后的 echostr 明文
func (h *CallbackHandler) VerifyURL(msgSignature, timestamp, nonce, echoStr string) (string, error) {
	if !verifySignature(h.client.Token, timestamp, nonce, echoStr, msgSignature) {
		return "", fmt.Errorf("signature verification failed")
	}
	decrypted, err := decryptWeComMsg(echoStr, h.client.AESKey)
	if err != nil {
		return "", fmt.Errorf("decrypt echostr: %w", err)
	}
	return string(decrypted), nil
}

// HandleCallback 处理企业微信消息回调（POST 请求处理）
func (h *CallbackHandler) HandleCallback(msgSignature, timestamp, nonce, body string) error {
	var encMsg WeComEncryptedMsg
	if err := xml.Unmarshal([]byte(body), &encMsg); err != nil {
		return fmt.Errorf("parse encrypted XML: %w", err)
	}
	if !verifySignature(h.client.Token, timestamp, nonce, encMsg.Encrypt, msgSignature) {
		return fmt.Errorf("signature verification failed")
	}
	decrypted, err := decryptWeComMsg(encMsg.Encrypt, h.client.AESKey)
	if err != nil {
		return fmt.Errorf("decrypt message: %w", err)
	}
	var msg WeComDecryptedMsg
	if err := xml.Unmarshal(decrypted, &msg); err != nil {
		return fmt.Errorf("parse decrypted XML: %w", err)
	}
	slog.Info("wecom message received (HandleCallback)",
		"from_user", msg.FromUserName,
		"msg_type", msg.MsgType,
	)
	if h.onMessage != nil {
		h.onMessage(msg.MsgType, msg.Content, msg.FromUserName)
	}
	return nil
}

// WeComEncryptedMsg 企业微信加密消息 XML
type WeComEncryptedMsg struct {
	XMLName    xml.Name `xml:"xml"`
	Encrypt    string   `xml:"Encrypt"`
	MsgSign    string   `xml:"MsgSignature"`
	TimeStamp  string   `xml:"TimeStamp"`
	Nonce      string   `xml:"Nonce"`
	ToUserName string   `xml:"ToUserName"`
	AgentID    string   `xml:"AgentID"`
}

// WeComDecryptedMsg 解密后的消息
type WeComDecryptedMsg struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgId        int64    `xml:"MsgId"`
	AgentID      int      `xml:"AgentID"`
}

// ServeHTTP 实现 http.Handler 接口
func (h *CallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.handleVerify(w, r)
		return
	}
	if r.Method == http.MethodPost {
		h.handleMessage(w, r)
		return
	}
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

// handleVerify URL 验证回调
func (h *CallbackHandler) handleVerify(w http.ResponseWriter, r *http.Request) {
	msgSignature := r.URL.Query().Get("msg_signature")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")
	echoStr := r.URL.Query().Get("echostr")

	// 验签
	if !verifySignature(h.client.Token, timestamp, nonce, echoStr, msgSignature) {
		slog.Warn("wecom callback verify: signature mismatch")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 解密 echostr
	decrypted, err := decryptWeComMsg(echoStr, h.client.AESKey)
	if err != nil {
		slog.Error("wecom callback verify: decrypt failed", "error", err)
		http.Error(w, "Decrypt Failed", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write(decrypted)
	slog.Info("wecom callback URL verified")
}

// handleMessage 消息回调
func (h *CallbackHandler) handleMessage(w http.ResponseWriter, r *http.Request) {
	msgSignature := r.URL.Query().Get("msg_signature")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 解析加密 XML
	var encMsg WeComEncryptedMsg
	if err := xml.Unmarshal(body, &encMsg); err != nil {
		slog.Error("wecom callback: parse XML failed", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 验签
	if !verifySignature(h.client.Token, timestamp, nonce, encMsg.Encrypt, msgSignature) {
		slog.Warn("wecom callback: signature mismatch")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 解密
	decrypted, err := decryptWeComMsg(encMsg.Encrypt, h.client.AESKey)
	if err != nil {
		slog.Error("wecom callback: decrypt failed", "error", err)
		http.Error(w, "Decrypt Failed", http.StatusInternalServerError)
		return
	}

	// 解析明文 XML
	var msg WeComDecryptedMsg
	if err := xml.Unmarshal(decrypted, &msg); err != nil {
		slog.Error("wecom callback: parse decrypted XML failed", "error", err)
		http.Error(w, "Parse Failed", http.StatusInternalServerError)
		return
	}

	slog.Info("wecom message received",
		"from_user", msg.FromUserName,
		"msg_type", msg.MsgType,
		"agent_id", msg.AgentID,
	)

	if h.onMessage != nil {
		h.onMessage(msg.MsgType, msg.Content, msg.FromUserName)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("success"))
}

// verifySignature 验证企业微信签名
// signature = SHA1(sort(token, timestamp, nonce, encrypt))
func verifySignature(token, timestamp, nonce, encrypt, expectedSig string) bool {
	if token == "" {
		return true // 未配置则跳过
	}
	strs := []string{token, timestamp, nonce, encrypt}
	sort.Strings(strs)
	joined := strings.Join(strs, "")
	hash := sha1.Sum([]byte(joined))
	computed := fmt.Sprintf("%x", hash)
	return computed == expectedSig
}

// decryptWeComMsg 解密企业微信消息
// AESKey = Base64Decode(EncodingAESKey + "="), IV = AESKey[:16]
func decryptWeComMsg(encrypt, encodingAESKey string) ([]byte, error) {
	if encodingAESKey == "" {
		return nil, fmt.Errorf("encodingAESKey is empty")
	}

	// Base64 解码 AES Key
	aesKey, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return nil, fmt.Errorf("decode AES key: %w", err)
	}

	// Base64 解码密文
	ciphertext, err := base64.StdEncoding.DecodeString(encrypt)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	// AES-256-CBC 解密
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	iv := aesKey[:aes.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// 去除 PKCS7 填充
	padLen := int(plaintext[len(plaintext)-1])
	if padLen < 1 || padLen > aes.BlockSize || padLen > len(plaintext) {
		return nil, fmt.Errorf("invalid PKCS7 padding")
	}
	plaintext = plaintext[:len(plaintext)-padLen]

	// 格式: 16 字节随机 + 4 字节消息长度 + 消息内容 + CorpID
	if len(plaintext) < 20 {
		return nil, fmt.Errorf("decrypted data too short")
	}
	msgLen := binary.BigEndian.Uint32(plaintext[16:20])
	if int(msgLen) > len(plaintext)-20 {
		return nil, fmt.Errorf("invalid message length")
	}

	return plaintext[20 : 20+msgLen], nil
}
