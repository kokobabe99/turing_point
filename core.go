package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

/* ===================== 基本枚举 ===================== */

type Move int8

const (
	L Move = -1
	R Move = +1
)

func dirStr(m Move) string {
	if m == L {
		return "L"
	}
	return "R"
}

type StepStatus int

const (
	Continue StepStatus = iota
	Accept
	Reject
)

type MachineKind int

const (
	KindTWA MachineKind = iota
	KindTM
	KindPDA
	KindTwoPDA
	KindTransducer
)

func machineKindToString(k MachineKind) string {
	switch k {
	case KindTWA:
		return "twa"
	case KindTM:
		return "tm"
	case KindPDA:
		return "pda"
	case KindTwoPDA:
		return "2pda"
	case KindTransducer:
		return "transducer"
	default:
		return "unknown"
	}
}

type Action int

const (
	ActNone Action = iota
	ActScan
	ActWriteTape
	ActPush1
	ActPop1
	ActPush2
	ActPop2
	ActPrint
)

/* ===================== 状态和规则 ===================== */

type State struct {
	id     int
	dir    Move
	next   map[uint8]*State
	accept bool
	reject bool

	action   Action
	writeSym byte
	stackSym byte // 对 Push/Pop 状态：要 push/pop 的符号
	printSym byte // Print 状态输出的符号
}

type rawLine struct {
	id     int
	dir    Move
	action Action
	pairs  [][2]string
	acc    bool
	rej    bool
	outSym byte
}

/* ===================== Machine 接口 ===================== */

type Machine interface {
	Kind() MachineKind
	Run(tape []byte) (bool, error)
	Dump()
	WriteDOT(path string) error
}

/* ===================== 解析工具 ===================== */

func parseDirWord(w string) (Move, bool) {
	switch strings.ToLower(strings.TrimSpace(w)) {
	case "left", "l":
		return L, true
	case "right", "r":
		return R, true
	default:
		return R, false
	}
}

func parseActionWord(w string) (Action, bool) {
	s := strings.ToLower(strings.TrimSpace(w))
	switch s {
	case "scan":
		return ActScan, true
	case "none":
		return ActNone, true
	case "print":
		return ActPrint, true

	// 兼容：PDA / 2PDA 写法
	case "write", "write1", "push", "push1":
		return ActPush1, true
	case "write2", "push2":
		return ActPush2, true
	case "read", "read1", "pop", "pop1":
		return ActPop1, true
	case "read2", "pop2":
		return ActPop2, true

	// TM
	case "write-tape":
		return ActWriteTape, true

	default:
		return ActNone, false
	}
}

func parseMode(s string) (Move, Action, error) {
	orig := s
	s = strings.TrimSpace(s)
	if s == "" {
		return R, ActScan, fmt.Errorf("empty mode")
	}
	s = strings.ReplaceAll(s, "-", " ")
	parts := strings.Fields(strings.ToLower(s))

	if len(parts) == 1 {
		if d, ok := parseDirWord(parts[0]); ok {
			return d, ActScan, nil
		}
		if a, ok := parseActionWord(parts[0]); ok {
			return R, a, nil
		}
		return R, ActScan, fmt.Errorf("unknown mode %q", orig)
	}

	a, okA := parseActionWord(parts[0])
	d, okD := parseDirWord(parts[1])
	if !okA || !okD {
		return R, ActScan, fmt.Errorf("bad mode %q", orig)
	}
	return d, a, nil
}

/* ===================== 规则解析 ===================== */

func parseRules(path string) ([]rawLine, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	var lines []rawLine
	maxID := 0
	sc := bufio.NewScanner(f)
	ln := 0

	for sc.Scan() {
		ln++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "# ") {
			continue
		}

		// accept / reject
		if i := strings.Index(line, "]"); i > 0 && strings.Contains(strings.ToLower(line), "accept") {
			id, e := strconv.Atoi(strings.TrimSpace(line[:i]))
			if e != nil {
				return nil, 0, fmt.Errorf("line %d: %v", ln, e)
			}
			lines = append(lines, rawLine{id: id, acc: true})
			if id > maxID {
				maxID = id
			}
			continue
		}
		if i := strings.Index(line, "]"); i > 0 && strings.Contains(strings.ToLower(line), "reject") {
			id, e := strconv.Atoi(strings.TrimSpace(line[:i]))
			if e != nil {
				return nil, 0, fmt.Errorf("line %d: %v", ln, e)
			}
			lines = append(lines, rawLine{id: id, rej: true})
			if id > maxID {
				maxID = id
			}
			continue
		}

		// 普通规则行
		parts := strings.SplitN(line, "]", 2)
		if len(parts) != 2 {
			return nil, 0, fmt.Errorf("line %d: bad syntax", ln)
		}
		id, e := strconv.Atoi(strings.TrimSpace(parts[0]))
		if e != nil {
			return nil, 0, fmt.Errorf("line %d: %v", ln, e)
		}
		rest := strings.TrimSpace(parts[1])

		lower := strings.ToLower(rest)
		if strings.HasPrefix(lower, "print") {
			after := strings.TrimSpace(rest[len("print"):]) // "(a,4)..."
			lp := strings.IndexByte(after, '(')
			rp := strings.IndexByte(after, ')')
			if lp < 0 || rp < 0 || rp < lp {
				return nil, 0, fmt.Errorf("line %d: bad print syntax", ln)
			}
			inside := strings.TrimSpace(after[lp+1 : rp])
			xy := strings.Split(inside, ",")
			if len(xy) != 2 {
				return nil, 0, fmt.Errorf("line %d: bad print pair", ln)
			}
			symStr := strings.TrimSpace(xy[0])
			nextStr := strings.TrimSpace(xy[1])

			if len(symStr) != 1 {
				return nil, 0, fmt.Errorf("line %d: print sym must be 1 char", ln)
			}
			if symStr[0] == '#' {
				return nil, 0, fmt.Errorf("line %d: print sym cannot be '#'", ln)
			}
			if _, e := strconv.Atoi(nextStr); e != nil {
				return nil, 0, fmt.Errorf("line %d: bad to-state %q", ln, nextStr)
			}

			rl := rawLine{
				id:     id,
				action: ActPrint,
				outSym: symStr[0],
			}
			rl.pairs = append(rl.pairs, [2]string{"_", nextStr})
			if v, _ := strconv.Atoi(nextStr); v > maxID {
				maxID = v
			}
			lines = append(lines, rl)
			if id > maxID {
				maxID = id
			}
			continue
		}

		lp := strings.IndexByte(rest, '(')
		if lp < 0 {
			return nil, 0, fmt.Errorf("line %d: missing '('", ln)
		}
		modeStr := strings.TrimSpace(rest[:lp])
		dir, action, e := parseMode(modeStr)
		if e != nil {
			return nil, 0, fmt.Errorf("line %d: %v", ln, e)
		}

		var pairs [][2]string
		right := rest[lp:]

		for {
			l := strings.IndexByte(right, '(')
			r := strings.IndexByte(right, ')')
			if l < 0 || r < 0 || r < l {
				break
			}
			inside := strings.TrimSpace(right[l+1 : r])
			right = right[r+1:]
			xy := strings.Split(inside, ",")
			if len(xy) != 2 {
				return nil, 0, fmt.Errorf("line %d: expect (sym,to)", ln)
			}
			sym := strings.TrimSpace(xy[0])
			to := strings.TrimSpace(xy[1])
			if len(sym) != 1 {
				return nil, 0, fmt.Errorf("line %d: bad symbol %q", ln, sym)
			}
			if _, e := strconv.Atoi(to); e != nil {
				return nil, 0, fmt.Errorf("line %d: bad to-state %q", ln, to)
			}
			pairs = append(pairs, [2]string{sym, to})
			if v, _ := strconv.Atoi(to); v > maxID {
				maxID = v
			}
		}

		lines = append(lines, rawLine{id: id, dir: dir, action: action, pairs: pairs})
		if id > maxID {
			maxID = id
		}
	}

	if e := sc.Err(); e != nil {
		return nil, 0, e
	}
	if maxID == 0 {
		return nil, 0, fmt.Errorf("no states parsed")
	}
	return lines, maxID, nil
}

/* ===================== 图构建 ===================== */

func buildStates(lines []rawLine, maxID int) ([]*State, *State, error) {
	used := make(map[int]bool)
	for _, ln := range lines {
		used[ln.id] = true
		for _, p := range ln.pairs {
			toID, _ := strconv.Atoi(p[1])
			used[toID] = true
		}
	}

	st := make([]*State, maxID+1)
	for id := range used {
		st[id] = &State{
			id:     id,
			dir:    R,
			action: ActScan,
		}
	}

	for _, ln := range lines {
		s := st[ln.id]
		if s == nil {
			continue
		}
		if ln.acc {
			s.accept = true
		}
		if ln.rej {
			s.reject = true
		}
		if len(ln.pairs) > 0 || ln.action != ActNone {
			s.dir = ln.dir
			if ln.action != ActNone {
				s.action = ln.action
			}
		}
		if ln.action == ActPrint {
			s.printSym = ln.outSym
		}
		// Push：用第一个 pair 符号作为 stackSym
		if (ln.action == ActPush1 || ln.action == ActPush2) && len(ln.pairs) > 0 {
			symStr := ln.pairs[0][0]
			if len(symStr) > 0 {
				s.stackSym = symStr[0]
			}
		}

		for _, p := range ln.pairs {
			toID, _ := strconv.Atoi(p[1])
			if st[toID] == nil {
				st[toID] = &State{id: toID, dir: R, action: ActScan}
			}
			if s.next == nil {
				s.next = make(map[uint8]*State)
			}
			key := p[0][0]
			s.next[key] = st[toID]
		}
	}

	start := st[1]
	if start == nil {
		return st, nil, fmt.Errorf("start state 1 not defined")
	}
	return st, start, nil
}

/* ===================== dump & DOT 共用 ===================== */

func actionName(a Action) string {
	switch a {
	case ActScan:
		return "Scan"
	case ActWriteTape:
		return "WTape"
	case ActPush1:
		return "Push1"
	case ActPop1:
		return "Pop1"
	case ActPush2:
		return "Push2"
	case ActPop2:
		return "Pop2"
	case ActPrint:
		return "Print"
	default:
		return "None"
	}
}

func dumpStates(states []*State) {
	fmt.Println("=== FSM (node graph) ===")
	for id := 1; id < len(states); id++ {
		s := states[id]
		if s == nil {
			continue
		}
		if len(s.next) == 0 && !s.accept && !s.reject {
			continue
		}
		tag := ""
		if s.accept {
			tag += " [ACCEPT]"
		}
		if s.reject {
			tag += " [REJECT]"
		}
		fmt.Printf("%d] dir=%s action=%s%s  ", s.id, dirStr(s.dir), actionName(s.action), tag)
		for key := range s.next {
			fmt.Printf("(%d->%c) ", s.id, key)
		}
		fmt.Println()
	}
}

func writeDOTCommon(states []*State, path string, showDir bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintln(f, "digraph FSM {")
	fmt.Fprintln(f, `  rankdir=LR; node [shape=circle, fontname="Arial"];`)
	for id := 1; id < len(states); id++ {
		s := states[id]
		if s == nil {
			continue
		}
		if len(s.next) == 0 && !s.accept && !s.reject {
			continue
		}
		shape := "circle"
		color := ""
		if s.accept {
			shape = "doublecircle"
			color = `, color="green"`
		}
		if s.reject {
			shape = "octagon"
			color = `, color="red"`
		}
		var lbl string
		if showDir {
			lbl = fmt.Sprintf("%d\\n[%s,%s]", s.id, actionName(s.action), dirStr(s.dir))
		} else {
			lbl = fmt.Sprintf("%d\\n[%s]", s.id, actionName(s.action))
		}
		fmt.Fprintf(f, "  %d [label=\"%s\", shape=%s%s];\n", s.id, lbl, shape, color)
		for key, value := range s.next {
			fmt.Fprintf(f, "  %d -> %d [label=\"%c\"];\n", s.id, value.id, key)
		}
	}
	fmt.Fprintln(f, "}")
	return nil
}

/* ===================== Tape 显示 ===================== */

func highlightIndex(tape string, head int) string {
	if head < 0 || head >= len(tape) {
		return tape
	}
	var b strings.Builder
	b.Grow(len(tape) + 2)
	b.WriteString(tape[:head])
	b.WriteByte('[')
	b.WriteByte(tape[head])
	b.WriteByte(']')
	if head+1 < len(tape) {
		b.WriteString(tape[head+1:])
	}
	return b.String()
}

func displayTapeWithHead(tape string, head int) {
	fmt.Println("Tape :", highlightIndex(tape, head))
}

/* ===================== Tape & Kind 解析 ===================== */

func parseTapeArg(arg string) ([]byte, error) {
	s := strings.TrimSpace(arg)
	if len(s) < 2 || s[0] != '#' || s[len(s)-1] != '#' {
		return nil, fmt.Errorf("tape must be wrapped with #...#")
	}
	return []byte(s), nil
}

func parseMachineKind(s string) (MachineKind, error) {
	k := strings.ToLower(strings.TrimSpace(s))
	switch k {
	case "twa":
		return KindTWA, nil
	case "tm":
		return KindTM, nil
	case "pda":
		return KindPDA, nil
	case "2pda", "two_pda", "twopda":
		return KindTwoPDA, nil
	case "trans", "transducer", "gtrans", "gt":
		return KindTransducer, nil
	default:
		return KindTWA, fmt.Errorf("unknown machine kind %q (use: twa|tm|pda|2pda|transducer)", s)
	}
}
