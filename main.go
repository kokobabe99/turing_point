package main

import (
	"fmt"
	"os"
	"path/filepath"

	"strings"
)

/* ===================== Machine 工厂 ===================== */

func newMachine(kind MachineKind, states []*State, start *State) (Machine, error) {
	switch kind {
	case KindTWA:
		return NewTWAMachine(states, start), nil
	case KindTM:
		return NewTMMachine(states, start), nil
	case KindPDA:
		return NewPDAMachine(states, start), nil
	case KindTwoPDA:
		return NewTwoPDAMachine(states, start), nil
	case KindTransducer:
		return NewTransducerMachine(states, start), nil
	default:
		return nil, fmt.Errorf("unsupported machine kind %v", kind)
	}
}

/* ===================== main ===================== */

func main() {
	var (
		kind      MachineKind
		rulesPath string
		tapeArg   string
	)

	if len(os.Args) == 3 {
		kind = KindTWA
		rulesPath = os.Args[1]
		tapeArg = os.Args[2]
	} else if len(os.Args) == 4 {
		k, err := parseMachineKind(os.Args[1])
		if err != nil {
			fmt.Println("kind error:", err)
			return
		}
		kind = k
		rulesPath = os.Args[2]
		tapeArg = os.Args[3]
	} else {
		fmt.Println("Usage:")
		fmt.Println("  go run main.go rules.txt \"#tape#\"")
		fmt.Println("  go run main.go <twa|tm|pda|2pda|transducer> rules.txt \"#tape#\"")
		return
	}

	raws, maxID, err := parseRules(rulesPath)
	if err != nil {
		fmt.Println("parse error:", err)
		return
	}
	states, start, err := buildStates(raws, maxID)
	if err != nil {
		fmt.Println("build error:", err)
		return
	}

	fmt.Println("=== EDGES ===")
	for id, s := range states {
		if s == nil {
			continue
		}
		for sym, to := range s.next {
			fmt.Printf("  %d --'%c'--> %d\n", id, sym, to.id)
		}
	}
	fmt.Println("=== END EDGES ===")

	m, err := newMachine(kind, states, start)
	if err != nil {
		fmt.Println("machine error:", err)
		return
	}

	m.Dump()

	base := filepath.Base(rulesPath)
	dotName := fmt.Sprintf("%s.dot", strings.ReplaceAll(base, ".txt", ""))
	if err := m.WriteDOT("dots" + "/" + dotName); err != nil {
		fmt.Println("dot error:", err)
		return
	}
	fmt.Println("DOT saved to:", dotName)

	tape, err := parseTapeArg(tapeArg)
	if err != nil {
		fmt.Println("tape error:", err)
		return
	}

	ok, err := m.Run(tape)
	if err != nil {
		fmt.Println("run error:", err)
		return
	}
	fmt.Printf("Final tape : %s\n", string(tape))
	fmt.Printf("Result     : %s\n", map[bool]string{true: "ACCEPT", false: "REJECT"}[ok])
}
