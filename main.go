package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

/* ===================== 基本方向 & 状态结果 ===================== */

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

/* ===================== 机器类型 & 动作类型 ===================== */

type MachineKind int

const (
	KindTWA        MachineKind = iota // Two-way automaton
	KindTM                            // Turing Machine
	KindPDA                           // one-way PDA
	KindTwoWayPDA                     // two-way PDA (这里当作 2-stack PDA)
	KindTransducer                    // generalized transducer
)

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

/* ===================== 状态定义 ===================== */

type State struct {
	id     int
	dir    Move
	next   map[uint8]*State
	accept bool
	reject bool

	action   Action
	writeSym byte // ActWriteTape（当前未使用）
	stackSym byte // Push/Pop 使用的栈符号
	printSym byte // ActPrint 输出的符号
}

/* ===================== rawLine：规则中间表示 ===================== */

type rawLine struct {
	id     int
	dir    Move
	action Action
	pairs  [][2]string // (key, toID)，Print 的 key 用 '_' 占位
	acc    bool
	rej    bool
	outSym byte // Print 输出的符号
}

/* ===================== 运行时环境 ===================== */

type Runtime struct {
	Tape   []byte
	Head   int
	Stack  []byte // 栈1
	Stack2 []byte // 栈2（2PDA 用）
	Output []byte
	Kind   MachineKind
}

/* ===================== Action / Direction 解析 ===================== */

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

	// 兼容旧的 Write/Read（默认栈1）
	case "write", "write1", "push", "push1":
		return ActPush1, true
	case "write2", "push2":
		return ActPush2, true

	case "read", "read1", "pop", "pop1":
		return ActPop1, true
	case "read2", "pop2":
		return ActPop2, true
	default:
		return ActNone, false
	}
}

// 解析 mode：
//   - "left" / "right"
//   - "scan-left" / "scan right"
//   - "scan" / "read"/"write"/"write1"/"write2"/"read1"/"read2"
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
			// 只有方向，默认 Scan
			return d, ActScan, nil
		}
		if a, ok := parseActionWord(parts[0]); ok {
			// 只有动作，方向默认 R
			return R, a, nil
		}
		return R, ActScan, fmt.Errorf("unknown mode %q", orig)
	}

	// 两个词：第一个动作，第二个方向
	a, okA := parseActionWord(parts[0])
	d, okD := parseDirWord(parts[1])
	if !okA || !okD {
		return R, ActScan, fmt.Errorf("bad mode %q", orig)
	}
	return d, a, nil
}

/* ===================== State 方法 ===================== */

func (s *State) nextOn(sym byte) (*State, error) {
	if state, ok := s.next[sym]; ok {
		return state, nil
	}
	return nil, fmt.Errorf("invalid symbol %q", sym)
}

func (s *State) Step(rt *Runtime) (*State, StepStatus, error) {
	displayTapeWithHead(string(rt.Tape), rt.Head)

	if rt.Head < 0 || rt.Head >= len(rt.Tape) {
		return nil, Reject, fmt.Errorf("head out of bounds: %d", rt.Head)
	}

	cur := rt.Tape[rt.Head]

	// ==== Transducer 在最后一个 '#'（非 Print 状态）提前结束 ====
	if rt.Kind == KindTransducer &&
		cur == '#' &&
		rt.Head == len(rt.Tape)-1 &&
		s.action != ActPrint {
		return s, Accept, nil
	}

	// ---------- Print 特殊处理 ----------
	if s.action == ActPrint {
		var nxt *State
		if s.next != nil {
			if v, ok := s.next['_']; ok {
				nxt = v
			} else if len(s.next) == 1 {
				for _, v := range s.next {
					nxt = v
					break
				}
			}
		}
		if nxt == nil {
			return nil, Reject, fmt.Errorf("print state %d missing next", s.id)
		}
		if s.printSym == 0 {
			return nil, Reject, fmt.Errorf("print state %d has no printSym", s.id)
		}

		rt.Output = append(rt.Output, s.printSym)

		if nxt.accept {
			return nxt, Accept, nil
		}
		if nxt.reject {
			return nxt, Reject, nil
		}
		// Print 不移动 head
		return nxt, Continue, nil
	}

	// ---------- 普通分支 ----------
	nxt, err := s.nextOn(cur)
	if err != nil {
		return nil, Reject, err
	}
	if nxt == nil {
		return nil, Reject, fmt.Errorf("missing transition: state %d on %q", s.id, cur)
	}

	// ---------- 执行动作 ----------
	switch s.action {
	case ActNone, ActScan:
		// nothing

	case ActWriteTape:
		rt.Tape[rt.Head] = s.writeSym

	case ActPush1:
		// 栈1：只在当前读到 stackSym 时 push
		if s.stackSym == 0 || cur == s.stackSym {
			rt.Stack = append(rt.Stack, s.stackSym)
		}

	case ActPop1:
		// PDA/2PDA：在 '#' 上不 pop，留给 empty-stack 检查
		if (rt.Kind == KindPDA || rt.Kind == KindTwoWayPDA) && cur == '#' {
			// do nothing
		} else {
			if len(rt.Stack) == 0 {
				return nxt, Reject, fmt.Errorf("stack1 underflow at state %d", s.id)
			}
			top := rt.Stack[len(rt.Stack)-1]
			if s.stackSym != 0 && top != s.stackSym {
				return nxt, Reject, fmt.Errorf("stack1 unexpected top %q at state %d", top, s.id)
			}
			rt.Stack = rt.Stack[:len(rt.Stack)-1]
		}

	case ActPush2:
		if s.stackSym == 0 || cur == s.stackSym {
			rt.Stack2 = append(rt.Stack2, s.stackSym)
		}

	case ActPop2:
		if (rt.Kind == KindPDA || rt.Kind == KindTwoWayPDA) && cur == '#' {
			// do nothing
		} else {
			if len(rt.Stack2) == 0 {
				return nxt, Reject, fmt.Errorf("stack2 underflow at state %d", s.id)
			}
			top := rt.Stack2[len(rt.Stack2)-1]
			if s.stackSym != 0 && top != s.stackSym {
				return nxt, Reject, fmt.Errorf("stack2 unexpected top %q at state %d", top, s.id)
			}
			rt.Stack2 = rt.Stack2[:len(rt.Stack2)-1]
		}
	}

	// ---------- 按机器类型移动 head ----------
	switch rt.Kind {
	case KindTWA, KindTM:
		if nxt.dir == L {
			rt.Head--
		} else {
			rt.Head++
		}

	case KindPDA:
		if cur != '#' {
			rt.Head++
		}

	case KindTwoWayPDA:
		// 2-stack PDA: one-way，只向右扫
		if cur != '#' {
			rt.Head++
		}

	case KindTransducer:
		if s.action == ActScan && cur != '#' {
			rt.Head++
		}

	default:
		rt.Head++
	}

	if rt.Head < 0 || rt.Head >= len(rt.Tape) {
		return nxt, Reject, fmt.Errorf("head moved out of tape: %d", rt.Head)
	}

	// ---------- 统一的 accept/reject 检查 ----------
	if nxt.accept {
		if rt.Kind == KindPDA {
			// 1-stack PDA：栈1空
			if len(rt.Stack) == 0 {
				return nxt, Accept, nil
			}
			return nxt, Reject, nil
		}
		if rt.Kind == KindTwoWayPDA {
			// 2-stack PDA：两个栈都空
			if len(rt.Stack) == 0 && len(rt.Stack2) == 0 {
				return nxt, Accept, nil
			}
			return nxt, Reject, nil
		}
		// 其他机器：final-state accept
		return nxt, Accept, nil
	}

	if nxt.reject {
		return nxt, Reject, nil
	}

	return nxt, Continue, nil
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

		// --- accept / reject ---
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

		// --- 普通规则行 ---
		parts := strings.SplitN(line, "]", 2)
		if len(parts) != 2 {
			return nil, 0, fmt.Errorf("line %d: bad syntax", ln)
		}
		id, e := strconv.Atoi(strings.TrimSpace(parts[0]))
		if e != nil {
			return nil, 0, fmt.Errorf("line %d: %v", ln, e)
		}
		rest := strings.TrimSpace(parts[1])

		// -------- Print 行： 2] Print (1,4) --------
		lower := strings.ToLower(rest)
		if strings.HasPrefix(lower, "print") {
			after := strings.TrimSpace(rest[len("print"):]) //  "(1,4) ..."

			lp := strings.IndexByte(after, '(')
			rp := strings.IndexByte(after, ')')
			if lp < 0 || rp < 0 || rp < lp {
				return nil, 0, fmt.Errorf("line %d: bad print syntax", ln)
			}
			inside := strings.TrimSpace(after[lp+1 : rp]) // "a,4"
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

		// -------- 普通模式行： Scan-left / Scan-right / Read / Write ... --------
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
			inside := strings.TrimSpace(right[l+1 : r]) // "a,2"
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

func buildGraph(lines []rawLine, maxID int) ([]*State, *State, error) {
	// 先收集真正用到的 id
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
		// Print 输出符号
		if ln.action == ActPrint {
			s.printSym = ln.outSym
		}
		// Push1 / Push2：自动记录第一个转移的输入符号为 stackSym
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
			key := p[0][0] // 普通规则：输入符号；Print：我们用 '_' 作为 key
			s.next[key] = st[toID]
		}
	}

	start := st[1]
	if start == nil {
		return st, nil, fmt.Errorf("start state 1 not defined")
	}
	return st, start, nil
}

/* ===================== 校验：PDA/2PDA Write 不能在 '#' 上 ===================== */

func validateNoWriteOnHash(states []*State, kind MachineKind) error {
	if kind != KindPDA && kind != KindTwoWayPDA {
		return nil
	}
	for _, s := range states {
		if s == nil {
			continue
		}
		if (s.action == ActPush1 || s.action == ActPush2) && s.next != nil {
			if _, hasHash := s.next['#']; hasHash {
				return fmt.Errorf("PDA/2PDA state %d: Write on '#' is not allowed", s.id)
			}
		}
	}
	return nil
}

/* ===================== DOT & dump ===================== */

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

func dump(states []*State) {
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

func writeDOT(states []*State, path string, kind MachineKind) error {
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
		if kind == KindTransducer || kind == KindTM || kind == KindTwoWayPDA {
			// TM / Transducer / 2PDA 都不显示方向
			lbl = fmt.Sprintf("%d\\n[%s]", s.id, actionName(s.action))
		} else {
			lbl = fmt.Sprintf("%d\\n[%s,%s]", s.id, actionName(s.action), dirStr(s.dir))
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

/* ===================== run ===================== */

func run(rt *Runtime, start *State) (bool, error) {
	q := start
	step := 1

	fmt.Println("== TRACE START ==")

	for {
		fmt.Printf("=============================================\n")
		nxt, st, err := q.Step(rt)
		if err != nil {
			return false, err
		}

		read := rt.Tape[rt.Head]

		fmt.Printf("step  state       read  next  move  head\n")
		fmt.Printf("%-5d %-10s  %-4s  %-4d  %-4s  %d\n",
			step,
			fmt.Sprintf("%d(%s)", q.id, dirStr(q.dir)),
			string(read),
			nxt.id,
			dirStr(nxt.dir),
			rt.Head,
		)

		switch st {
		case Accept:
			return true, nil
		case Reject:
			return false, nil
		default:
			q = nxt
			step++
		}
		time.Sleep(300 * time.Millisecond)
	}
}

/* ===================== tape & kind 解析 ===================== */

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
	case "2pda", "two_way_pda", "twowaypda":
		return KindTwoWayPDA, nil
	case "trans", "transducer", "gtrans", "gt":
		return KindTransducer, nil
	default:
		return KindTWA, fmt.Errorf("unknown machine kind %q (use: twa|tm|pda|2pda|transducer)", s)
	}
}

/* ===================== main ===================== */

func main() {
	var (
		kind      MachineKind
		rulesPath string
		tapeArg   string
	)

	// 支持：
	//  - go run main.go rules.txt "#tape#"
	//  - go run main.go twa rules.txt "#tape#"
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

	states, start, err := buildGraph(raws, maxID)
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

	if err := validateNoWriteOnHash(states, kind); err != nil {
		fmt.Println("validation error:", err)
		return
	}

	dump(states)

	base := filepath.Base(rulesPath)
	dotName := fmt.Sprintf("%s.dot", strings.ReplaceAll(base, ".txt", ""))
	_ = os.MkdirAll("dots", 0755)
	if err := writeDOT(states, "dots/"+dotName, kind); err != nil {
		fmt.Println("dot error:", err)
		return
	}
	fmt.Println("DOT saved to:", dotName)

	tape, err := parseTapeArg(tapeArg)
	if err != nil {
		fmt.Println("tape error:", err)
		return
	}

	rt := &Runtime{
		Tape: tape,
		Head: 1, // 跳过左侧 '#'
		Kind: kind,
	}

	ok, err := run(rt, start)
	if err != nil {
		fmt.Println("run error:", err)
		return
	}

	fmt.Printf("Final tape : %s\n", string(rt.Tape))
	fmt.Printf("Result     : %s\n", map[bool]string{true: "ACCEPT", false: "REJECT"}[ok])

	if kind == KindTransducer {
		fmt.Printf("Output tape: %s\n", string(rt.Output))
	}
}
