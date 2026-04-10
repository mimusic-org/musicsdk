//go:build wasip1

package musicsdk

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

// wy 平台加密相关常量
const (
	eapiKey    = "e82ckenh8dichen8"
	weApiNonce = "0CoJUm6Qyw8W8jud"
	weApiIv    = "0102030405060708"

	// RSA 公钥参数（与 netease/crypto.go 一致）
	weApiPubModulus = "00e0b509f6259df8642dbc35662901477df22677ec152b5ff68ace615bb7b725152b3ab17a876aea8a5aa76d2e417629ec4ee341f56135fccf695280104e0312ecbda92557c93870114af6c9d05c4f7f0c3685b7a46bee255932575cce10b424d813cfe4875d3e82047b97ddef52741d546b8e289dc6935b3ece0462db0a22b8e7"
	weApiPubKey     = "010001"

	// Linux API Key (Hex)
	linuxApiKeyHex = "7246674226682325323F5E6544673A51"
)

// --- 辅助函数 ---

// pkcs7Pad PKCS7 填充
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := make([]byte, padding)
	for i := range padtext {
		padtext[i] = byte(padding)
	}
	return append(data, padtext...)
}

// randomString 生成指定长度随机字符串
func randomString(size int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, size)
	rand.Read(b)
	result := make([]byte, size)
	for i, v := range b {
		result[i] = letters[int(v)%len(letters)]
	}
	return string(result)
}

// reverseString 反转字符串（RSA 加密需要）
func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// --- AES 加密实现 ---

// aesECBEncrypt AES-128-ECB 加密（带 PKCS7 填充）
func aesECBEncrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	blockSize := block.BlockSize()
	plaintext = pkcs7Pad(plaintext, blockSize)

	ciphertext := make([]byte, len(plaintext))
	for i := 0; i < len(plaintext); i += blockSize {
		block.Encrypt(ciphertext[i:i+blockSize], plaintext[i:i+blockSize])
	}

	return ciphertext, nil
}

// aesCBCEncrypt AES-128-CBC 加密（带 PKCS7 填充），返回 base64 编码
func aesCBCEncrypt(text string, key string, iv string) (string, error) {
	keyBytes := []byte(key)
	ivBytes := []byte(iv)
	srcBytes := []byte(text)

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}

	srcBytes = pkcs7Pad(srcBytes, block.BlockSize())
	blockMode := cipher.NewCBCEncrypter(block, ivBytes)
	crypted := make([]byte, len(srcBytes))
	blockMode.CryptBlocks(crypted, srcBytes)

	return base64.StdEncoding.EncodeToString(crypted), nil
}

// --- RSA 加密实现 ---

// rsaEncrypt RSA 加密（NoPadding，使用 math/big 大数运算，WASM 兼容）
// 算法: pow(int(hex(reverse(text))), pubKey, modulus)
func rsaEncrypt(text, pubKey, modulus string) string {
	// 1. 反转字符串
	text = reverseString(text)
	// 2. 转为 hex
	hexText := hex.EncodeToString([]byte(text))

	// 3. 大数运算
	biText := new(big.Int)
	biText.SetString(hexText, 16)

	biPub := new(big.Int)
	biPub.SetString(pubKey, 16)

	biMod := new(big.Int)
	biMod.SetString(modulus, 16)

	// exp = text^pub % mod
	biRet := new(big.Int).Exp(biText, biPub, biMod)

	// 4. 补齐 256 位 hex
	return fmt.Sprintf("%0256x", biRet)
}

// --- 对外暴露的加密方法 ---

// eapiEncrypt EAPI 加密函数
// 算法：
// 1. text = JSON 序列化(object)
// 2. message = "nobody" + url + "use" + text + "md5forencrypt"
// 3. digest = MD5(message) → hex 小写
// 4. data = url + "-36cd479b6b5-" + text + "-36cd479b6b5-" + digest
// 5. encrypted = AES-128-ECB(data, key)
// 6. params = hex 大写(encrypted)
func eapiEncrypt(url string, object interface{}) (string, error) {
	text, err := json.Marshal(object)
	if err != nil {
		return "", fmt.Errorf("marshal object: %w", err)
	}
	textStr := string(text)

	message := "nobody" + url + "use" + textStr + "md5forencrypt"
	hash := md5.Sum([]byte(message))
	digest := hex.EncodeToString(hash[:])

	data := url + "-36cd479b6b5-" + textStr + "-36cd479b6b5-" + digest

	encrypted, err := aesECBEncrypt([]byte(data), []byte(eapiKey))
	if err != nil {
		return "", fmt.Errorf("aes encrypt: %w", err)
	}

	return strings.ToUpper(hex.EncodeToString(encrypted)), nil
}

// weapiEncrypt WeAPI 加密函数
// 算法：
// 1. 生成随机 16 位 secKey
// 2. 第一次 AES-CBC 加密 (text + nonce)
// 3. 第二次 AES-CBC 加密 (第一次结果 + secKey)
// 4. RSA 加密 secKey
// 返回 params 和 encSecKey
func weapiEncrypt(object interface{}) (params string, encSecKey string, err error) {
	text, err := json.Marshal(object)
	if err != nil {
		return "", "", fmt.Errorf("marshal object: %w", err)
	}

	secKey := randomString(16)

	// 第一次 AES-CBC 加密
	encText, err := aesCBCEncrypt(string(text), weApiNonce, weApiIv)
	if err != nil {
		return "", "", fmt.Errorf("first aes encrypt: %w", err)
	}

	// 第二次 AES-CBC 加密
	params, err = aesCBCEncrypt(encText, secKey, weApiIv)
	if err != nil {
		return "", "", fmt.Errorf("second aes encrypt: %w", err)
	}

	// RSA 加密 secKey
	encSecKey = rsaEncrypt(secKey, weApiPubKey, weApiPubModulus)

	return params, encSecKey, nil
}

// linuxapiEncrypt Linux API 加密函数
// 算法：AES-128-ECB 加密，key 为 linuxApiKeyHex 解码后的字节
func linuxapiEncrypt(object interface{}) (string, error) {
	text, err := json.Marshal(object)
	if err != nil {
		return "", fmt.Errorf("marshal object: %w", err)
	}

	key, err := hex.DecodeString(linuxApiKeyHex)
	if err != nil {
		return "", fmt.Errorf("decode linux api key: %w", err)
	}

	encrypted, err := aesECBEncrypt([]byte(text), key)
	if err != nil {
		return "", fmt.Errorf("aes encrypt: %w", err)
	}

	return strings.ToUpper(hex.EncodeToString(encrypted)), nil
}
