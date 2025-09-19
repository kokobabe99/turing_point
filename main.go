package main

import (
	"fmt"
	"os"
	"strings"
)

func parseTapeArg(arg string) (string, error) {
	s := strings.TrimSpace(arg)

	// 必须以 # 开始并以 # 结束
	if len(s) < 2 || s[0] != '#' || s[len(s)-1] != '#' {
		return "", fmt.Errorf("tape must be wrapped with #...#")
	}
	// 只允许 a / b / #
	//for i := 0; i < len(s); i++ {
	//	if s[i] != 'a' && s[i] != 'b' && s[i] != '#' {
	//		return "", fmt.Errorf("tape must contain only a/b/#")
	//	}
	//}
	return s, nil // 原样返回，不截取 #
}

func main() {

	if len(os.Args) != 3 {
		fmt.Println("Usage: go run main.go <rules.txt> <tape or #tape#>")
		return
	}
	rulesPath := os.Args[1]
	tapeArg := os.Args[2]

	raws, maxID, err := parseRules(rulesPath)
	if err != nil {
		fmt.Println("parse error:", err)
		return
	}

	states, start, err := buildGraph(raws, maxID)
	if err != nil {
		fmt.Println("build error:", err)
		return
	}

	dump(states)

	if err := writeDOT(states, "fsm.dot"); err != nil {
		fmt.Println("dot error:", err)
		return
	}

	fmt.Println("DOT saved to: fsm.dot")

	tape, err := parseTapeArg(tapeArg)
	if err != nil {
		fmt.Println("tape error:", err)
		return
	}

	ok, err := run(tape, start)
	if err != nil {
		fmt.Println("run error:", err)
		return
	}

	fmt.Printf("Final: %s  =>  %s\n", tape, map[bool]string{true: "ACCEPT", false: "REJECT"}[ok])
}
