package router

import (
	"fmt"
	"strings"
)

// splitShellWords 按 POSIX shell 的词法规则拆分命令行：
// 空白分隔；单引号内完全字面；双引号内除 \" 和 \\ 外字面；
// 引号外反斜杠转义下一个字符。不允许未闭合的引号。
//
// 拆分结果用于直接 exec（不经过 shell），因此 shell 元字符
// （; | & > < ` $ 等）在结果中只是普通字符，不构成注入风险。
func splitShellWords(input string) ([]string, error) {
	var words []string
	var cur strings.Builder
	hasContent := false // 区分空字符串参数与无参数（"" 应产生一个空 token）

	i := 0
	n := len(input)
	for i < n {
		c := input[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if hasContent {
				words = append(words, cur.String())
				cur.Reset()
				hasContent = false
			}
			i++
		case c == '\'':
			// 单引号：完全字面直到闭合
			hasContent = true
			j := i + 1
			for j < n && input[j] != '\'' {
				j++
			}
			if j >= n {
				return nil, fmt.Errorf("未闭合的单引号")
			}
			cur.WriteString(input[i+1 : j])
			i = j + 1
		case c == '"':
			// 双引号：仅 \" 和 \\ 转义，其余字面
			hasContent = true
			j := i + 1
			closed := false
			for j < n {
				if input[j] == '\\' && j+1 < n && (input[j+1] == '"' || input[j+1] == '\\') {
					cur.WriteByte(input[j+1])
					j += 2
					continue
				}
				if input[j] == '"' {
					closed = true
					break
				}
				cur.WriteByte(input[j])
				j++
			}
			if !closed {
				return nil, fmt.Errorf("未闭合的双引号")
			}
			i = j + 1
		case c == '\\':
			if i+1 >= n {
				return nil, fmt.Errorf("末尾孤立的反斜杠")
			}
			cur.WriteByte(input[i+1])
			hasContent = true
			i += 2
		default:
			cur.WriteByte(c)
			hasContent = true
			i++
		}
	}
	if hasContent {
		words = append(words, cur.String())
	}
	return words, nil
}
