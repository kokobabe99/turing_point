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
	KindTWA        MachineKind = iota // TWA (scan-left/right)
	KindTM                            // Turing Machine
	KindPDA                           // one-way PDA
	KindTwoWayPDA                     // two-way PDA
	KindTransducer                    // generalized transducer
)

type Action int

const (
	ActNone      Action = iota // 什么都不做
	ActScan                    // 仅扫描输入带
	ActWriteTape               // TM：写输入/工作带
	ActPushStack               // PDA：push 栈
	ActPopStack                // PDA：pop 栈
	ActPrint                   // Transducer：输出到输出带
)

/* ===================== 状态定义 ===================== */

type State struct {
	id     int
	dir    Move
	next   map[uint8]*State
	accept bool
	reject bool

	// 扩展：动作 / 写什么 / 栈操作 / 打印什么
	action   Action
	writeSym byte // ActWriteTape
	stackSym byte // ActPushStack / ActPopStack
	printSym byte // ActPrint
}

type rawLine struct {
	id     int
	dir    Move
	action Action
	pairs  [][2]string
	acc    bool
	rej    bool
}

/* ===================== 运行时环境 ===================== */

type Runtime struct {
	Tape   []byte
	Head   int
	Stack  []byte
	Output []byte
	Kind   MachineKind
}

/* ===================== Action / Direction 解析 ===================== */

func parseDirWord(w string) (Move, bool) {
	switch strings.ToLower(w) {
	case "left", "l":
		return L, true
	case "right", "r":
		return R, true
	default:
		return R, false
	}
}

func parseActionWord(w string) (Action, bool) {
	switch strings.ToLower(w) {
	case "scan":
		return ActScan, true
	case "read":
		return ActPopStack, true
	case "write":
		return ActPushStack, true
	case "print":
		return ActPrint, true
	case "none":
		return ActNone, true
	default:
		return ActNone, false
	}
}

// 解析像：
//
//	"left" / "right"
//	"scan-left" / "scan right"
//	"scan" / "read" / "write" / "print"
func parseMode(s string) (Move, Action, error) {
	orig := s
	s = strings.TrimSpace(s)
	if s == "" {
		return R, ActScan, fmt.Errorf("empty mode")
	}

	// 把 "-" 当作空格；支持 "Scan-left" / "Scan left"
	s = strings.ReplaceAll(s, "-", " ")
	parts := strings.Fields(strings.ToLower(s))

	if len(parts) == 1 {
		// 单词：可能是方向，也可能是动作
		if d, ok := parseDirWord(parts[0]); ok {
			// 只给了方向，当作 "Scan-<dir>"
			return d, ActScan, nil
		}
		if a, ok := parseActionWord(parts[0]); ok {
			// 只给了动作，方向默认 R（对大部分情况无害）
			return R, a, nil
		}
		return R, ActScan, fmt.Errorf("unknown mode %q", orig)
	}

	// 两个及以上单词：默认第一个是动作，第二个是方向
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

	nxt, err := s.nextOn(cur)
	if err != nil {
		return nil, Reject, err
	}
	if nxt == nil {
		return nil, Reject, fmt.Errorf("missing transition: state %d on %q", s.id, cur)
	}

	// 到达 accept / reject 就停
	if nxt.accept {
		return nxt, Accept, nil
	}
	if nxt.reject {
		return nxt, Reject, nil
	}

	// 执行“当前状态”的动作
	switch s.action {
	case ActNone, ActScan:
		// 不改任何东西
	case ActWriteTape:
		rt.Tape[rt.Head] = s.writeSym
	case ActPushStack:
		rt.Stack = append(rt.Stack, s.stackSym)
	case ActPopStack:
		if len(rt.Stack) == 0 {
			return nxt, Reject, fmt.Errorf("stack underflow at state %d", s.id)
		}
		top := rt.Stack[len(rt.Stack)-1]
		if s.stackSym != 0 && top != s.stackSym {
			return nxt, Reject, fmt.Errorf("unexpected stack top %q at state %d", top, s.id)
		}
		rt.Stack = rt.Stack[:len(rt.Stack)-1]
	case ActPrint:
		if s.printSym != 0 {
			rt.Output = append(rt.Output, s.printSym)
		} else {
			rt.Output = append(rt.Output, cur)
		}
	}

	// 根据“机器类型 + 当前状态动作”决定是否移动 head
	switch rt.Kind {
	case KindTWA, KindTM:
		// 每一步都移动，方向由“下一个状态”的 dir 决定（保持原有设计）
		if nxt.dir == L {
			rt.Head--
		} else {
			rt.Head++
		}

	case KindPDA:
		// 只有 Scan 的时候才消费一个输入符号（右移）
		if s.action == ActScan {
			rt.Head++
		}

	case KindTwoWayPDA:
		// 只有 Scan 的时候才移动；方向由“下一个状态”的 dir 决定
		if s.action == ActScan {
			if nxt.dir == L {
				rt.Head--
			} else {
				rt.Head++
			}
		}

	case KindTransducer:
		// 只有 Scan 的时候才往右扫；Print 不动头，只往输出带写
		if s.action == ActScan {
			rt.Head++
		}

	default:
		rt.Head++
	}

	return nxt, Continue, nil
}

/* ===================== 规则解析（支持 Scan-left 等） ===================== */

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
		// q] accept / reject
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

		// q] <mode> (x,y) (x,y) ...
		parts := strings.SplitN(line, "]", 2)
		if len(parts) != 2 {
			return nil, 0, fmt.Errorf("line %d: bad syntax", ln)
		}
		id, e := strconv.Atoi(strings.TrimSpace(parts[0]))
		if e != nil {
			return nil, 0, fmt.Errorf("line %d: %v", ln, e)
		}
		rest := strings.TrimSpace(parts[1])

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

func buildGraph(lines []rawLine, maxID int) ([]*State, *State, error) {

	st := make([]*State, maxID+1)
	for i := 0; i <= maxID; i++ {
		st[i] = &State{
			id:     i,
			dir:    R,       // 默认向右
			action: ActScan, // 默认 Scan
		}
	}

	for _, ln := range lines {
		s := st[ln.id]
		if ln.acc {
			s.accept = true
		}
		if ln.rej {
			s.reject = true
		}
		if len(ln.pairs) > 0 {
			s.dir = ln.dir
			s.action = ln.action
		}
		for _, p := range ln.pairs {
			toID, _ := strconv.Atoi(p[1])
			if s.next == nil {
				s.next = make(map[uint8]*State)
			}
			s.next[p[0][0]] = st[toID]
		}
	}
	return st, st[1], nil
}

/* ===================== DOT & dump ===================== */

func dump(states []*State) {
	fmt.Println("=== FSM (node graph) ===")
	for id := 1; id < len(states); id++ {
		s := states[id]
		if s == nil {
			continue
		}
		tag := ""
		if s.accept {
			tag += " [ACCEPT]"
		}
		if s.reject {
			tag += " [REJECT]"
		}
		fmt.Printf("%d] dir=%s action=%d%s  ", s.id, dirStr(s.dir), s.action, tag)
		for key := range s.next {
			fmt.Printf("(%d->%c) ", s.id, key)
		}
		fmt.Println()
	}
}

func writeDOT(states []*State, path string) error {
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
		lbl := fmt.Sprintf("%d\\n[%s]", s.id, dirStr(s.dir))
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
		time.Sleep(500 * time.Millisecond)
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
	//  - 老接口：main rules.txt "#1010#"  => 默认为 TWA
	//  - 新接口：main kind rules.txt "#1010#"
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

	dump(states)

	base := filepath.Base(rulesPath)
	dotName := fmt.Sprintf("%s.dot", strings.ReplaceAll(base, ".txt", ""))
	if err := writeDOT(states, "dots"+"/"+dotName); err != nil {
		fmt.Println("dot error:", err)
		return
	}
	fmt.Println("DOT saved to: ", dotName)

	tape, err := parseTapeArg(tapeArg)
	if err != nil {
		fmt.Println("tape error:", err)
		return
	}

	rt := &Runtime{
		Tape: tape,
		Head: 1, // 跳过左边 '#'
		Kind: kind,
	}

	// 如果你想针对某种机器，给某些状态手动配置 writeSym / stackSym / printSym，
	// 可以在这里按 id 修改，比如：
	//
	// if kind == KindTransducer {
	//     // 例：让状态 2 的 Print 总是打印 'X'
	//     if len(states) > 2 {
	//         states[2].action = ActPrint
	//         states[2].printSym = 'X'
	//     }
	// }

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
