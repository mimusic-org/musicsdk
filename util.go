//go:build wasip1

package musicsdk

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// FormatPlayTime 将秒数格式化为 "MM:SS"
func FormatPlayTime(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	minutes := seconds / 60
	secs := seconds % 60
	return fmt.Sprintf("%02d:%02d", minutes, secs)
}

// FormatSingers 将歌手名数组拼接为 "A、B、C"
func FormatSingers(names []string) string {
	return strings.Join(names, "、")
}

// DecodeName 解码 HTML 实体（&amp; &lt; &gt; &quot; &#xx;）
func DecodeName(s string) string {
	// 常见 HTML 实体
	replacements := map[string]string{
		"&amp;":  "&",
		"&lt;":   "<",
		"&gt;":   ">",
		"&quot;": "\"",
		"&apos;": "'",
		"&nbsp;": " ",
	}

	result := s
	for entity, char := range replacements {
		result = strings.ReplaceAll(result, entity, char)
	}

	// 处理数字字符引用 &#xx; 和 &#xNN;
	result = decodeNumericEntities(result)

	return result
}

// decodeNumericEntities 解码数字字符引用
func decodeNumericEntities(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if i+2 < len(s) && s[i] == '&' && s[i+1] == '#' {
			// 查找分号
			end := i + 2
			for end < len(s) && end < i+10 && s[end] != ';' {
				end++
			}
			if end < len(s) && s[end] == ';' {
				numStr := s[i+2 : end]
				var codePoint int64
				var err error

				if len(numStr) > 0 && (numStr[0] == 'x' || numStr[0] == 'X') {
					// 十六进制 &#xNN;
					codePoint, err = strconv.ParseInt(numStr[1:], 16, 32)
				} else {
					// 十进制 &#NN;
					codePoint, err = strconv.ParseInt(numStr, 10, 32)
				}

				if err == nil && codePoint > 0 && codePoint < 0x110000 {
					result.WriteRune(rune(codePoint))
					i = end + 1
					continue
				}
			}
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}

// SizeToStr 文件大小格式化
func SizeToStr(size int64) string {
	if size < 0 {
		return "0B"
	}

	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2fGB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.2fMB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.2fKB", float64(size)/KB)
	default:
		return fmt.Sprintf("%dB", size)
	}
}

// TrimString 去除字符串两端空白
func TrimString(s string) string {
	return strings.TrimSpace(s)
}

// IsEmpty 判断字符串是否为空
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// FormatPlayCount 格式化播放量
// 例如: 12345 → "1.2万", 1234567 → "123.4万", 123456789 → "1.2亿"
func FormatPlayCount(num int) string {
	if num < 10000 {
		return fmt.Sprintf("%d", num)
	}
	if num < 100000000 {
		return fmt.Sprintf("%.1f万", float64(num)/10000)
	}
	return fmt.Sprintf("%.1f亿", float64(num)/100000000)
}

// DateFormat 将 Unix 时间戳（秒）格式化为日期字符串
func DateFormat(timestamp int64) string {
	if timestamp <= 0 {
		return ""
	}
	// 如果时间戳是毫秒级（大于 10^12），转换为秒
	if timestamp > 1e12 {
		timestamp = timestamp / 1000
	}
	t := time.Unix(timestamp, 0)
	return t.Format("2006-01-02")
}
