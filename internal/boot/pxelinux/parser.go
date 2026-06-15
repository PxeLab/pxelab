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
		case "ontimeout":
			ast.OnTimeout = strings.Join(args, " ")
		case "onerror":
			ast.OnError = strings.Join(args, " ")
		case "menu":
			handleMenuCommand(ast, args)
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
		case "menu label":
			if currentLabel != nil {
				currentLabel.MenuLabel = strings.Join(args, " ")
			}
		}
	}

	if currentLabel != nil {
		ast.Labels = append(ast.Labels, *currentLabel)
	}

	return ast, nil
}

func handleMenuCommand(ast *AST, args []string) {
	if len(args) >= 2 && strings.ToLower(args[0]) == "title" {
		ast.MenuTitle = strings.Join(args[1:], " ")
	}
}
