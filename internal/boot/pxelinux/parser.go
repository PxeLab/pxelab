package pxelinux

import (
	"bufio"
	"fmt"
	"strings"
)

func Parse(data string) (*AST, error) {
	ast := &AST{}
	scanner := bufio.NewScanner(strings.NewReader(data))
	var currentLabel *Label

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		tokens := strings.Fields(line)
		if len(tokens) == 0 {
			continue
		}

		cmd := strings.ToLower(tokens[0])
		args := tokens[1:]

		switch cmd {
		case "default":
			ast.Default = strings.Join(args, " ")
		case "timeout":
			fmt.Sscanf(strings.Join(args, " "), "%d", &ast.Timeout)
		case "prompt":
			var v int
			if fmt.Sscanf(strings.Join(args, " "), "%d", &v); v != 0 {
				ast.Prompt = true
			}
		case "ontimeout":
			ast.OnTimeout = strings.Join(args, " ")
		case "onerror":
			ast.OnError = strings.Join(args, " ")
		case "serial", "console":
			// 暂时忽略串口/控制台指令
		case "include":
			// 暂时忽略 include（未实现多文件解析）
		case "ipappend":
			// 暂时忽略（iPXE 动态判断架构）
		case "menu":
			handleMenuCommand(ast, currentLabel, args)
		case "label":
			if currentLabel != nil {
				ast.Labels = append(ast.Labels, *currentLabel)
			}
			currentLabel = &Label{Name: strings.Join(args, " ")}
		case "kernel":
			if currentLabel != nil {
				currentLabel.Kernel = strings.Join(args, " ")
			}
		case "append":
			if currentLabel != nil {
				currentLabel.Append = args
			}
		case "initrd":
			if currentLabel != nil {
				currentLabel.Initrd = strings.Join(args, " ")
			}
		case "local":
			if currentLabel != nil {
				currentLabel.Kernel = ""
			}
		}
	}

	if currentLabel != nil {
		ast.Labels = append(ast.Labels, *currentLabel)
	}

	// 填充 Defaults（默认 label 的配置摘要）
	for _, label := range ast.Labels {
		if label.Name == ast.Default {
			ast.Defaults = label
			break
		}
	}

	return ast, nil
}

func handleMenuCommand(ast *AST, currentLabel *Label, args []string) {
	if len(args) == 0 {
		return
	}
	sub := strings.ToLower(args[0])
	rest := args[1:]

	switch sub {
	case "title":
		ast.MenuTitle = strings.Join(rest, " ")
	case "default":
		if currentLabel != nil && len(rest) == 0 {
			ast.Default = currentLabel.Name
		} else if len(rest) > 0 {
			ast.Default = strings.Join(rest, " ")
		}
	case "label":
		if currentLabel != nil {
			currentLabel.MenuLabel = strings.Join(rest, " ")
		}
	case "indent":
		if currentLabel != nil && len(rest) > 0 {
			fmt.Sscanf(rest[0], "%d", &currentLabel.MenuIndent)
		}
	case "separator":
		// 在 iPXE 中用空 item 表示分隔线
		sep := Label{Name: "", MenuLabel: "---"}
		ast.Labels = append(ast.Labels, sep)
	case "hide":
		if currentLabel != nil {
			// PXELinux 的 menu hide，在 ipxe 中暂不处理
		}
	}
}
