package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func (m Move) String() string {
	if m == L {
		return "L"
	}
	return "R"
}

func parseMoveLR(s string) (Move, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "left", "l":
		return L, true
	case "right", "r":
		return R, true
	default:
		return 0, false // 不支持 STAY
	}
}

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
		if i := strings.Index(line, "]"); i > 0 && strings.Contains(line, "accept") {
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
		if i := strings.Index(line, "]"); i > 0 && strings.Contains(line, "reject") {
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

		// q] left|right (x,y) (x,y) ...
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
		dirStr := strings.TrimSpace(rest[:lp])
		dir, ok := parseMoveLR(dirStr)
		if !ok {
			return nil, 0, fmt.Errorf("line %d: move must be left/right, got %q", ln, dirStr)
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
			if len(sym) != 1 || (sym[0] != '#' && sym[0] != 'a' && sym[0] != 'b') {
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
		lines = append(lines, rawLine{id: id, dir: dir, pairs: pairs})
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
		st[i] = &State{id: i, dir: R}
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
		}
		for _, p := range ln.pairs {
			toID, _ := strconv.Atoi(p[1])
			switch p[0][0] {
			case '#':
				s.onHash = st[toID]
			case 'a':
				s.onA = st[toID]
			case 'b':
				s.onB = st[toID]
			}
		}
	}
	return st, st[1], nil
}

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
		fmt.Printf("%d] dir=%s%s  ", s.id, s.dir, tag)
		if s.onA != nil {
			fmt.Printf("(a->%d) ", s.onA.id)
		}
		if s.onB != nil {
			fmt.Printf("(b->%d) ", s.onB.id)
		}
		if s.onHash != nil {
			fmt.Printf("(#->%d) ", s.onHash.id)
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
		lbl := fmt.Sprintf("%d\\n[%s]", s.id, s.dir)
		fmt.Fprintf(f, "  %d [label=\"%s\", shape=%s%s];\n", s.id, lbl, shape, color)
		if s.onA != nil {
			fmt.Fprintf(f, "  %d -> %d [label=\"a\"];\n", s.id, s.onA.id)
		}
		if s.onB != nil {
			fmt.Fprintf(f, "  %d -> %d [label=\"b\"];\n", s.id, s.onB.id)
		}
		if s.onHash != nil {
			fmt.Fprintf(f, "  %d -> %d [label=\"#\"];\n", s.id, s.onHash.id)
		}
	}
	fmt.Fprintln(f, "}")
	return nil
}

func displayTapeWithHead(tape string, head int) {
	disp := "#" + tape + "#"
	caretPos := head + 1 // head=-1 -> 0; head=len -> len
	if caretPos < 0 {
		caretPos = 0
	}
	if caretPos > len(disp)-1 {
		caretPos = len(disp) - 1
	}
	fmt.Println("Tape :", disp)
	fmt.Print("       ")
	for i := 0; i < caretPos; i++ {
		fmt.Print(" ")
	}
	fmt.Println("^")
}

func runTrace(tape string, start *State, stepCap int) (bool, error) {
	i := 0
	q := start

	fmt.Println("== TRACE START ==")
	displayTapeWithHead(tape, i)
	fmt.Printf("step  state(dir)  read  next  move  head\n")

	for step := 0; step < stepCap; step++ {
		if q.accept {
			fmt.Println("== ACCEPT ==")
			return true, nil
		}
		if q.reject {
			fmt.Println("== REJECT ==")
			return false, nil
		}

		var nxt *State
		read := byte('#')
		if i >= 0 && i < len(tape) {
			read = tape[i]
			switch read {
			case 'a':
				nxt = q.onA
			case 'b':
				nxt = q.onB
			default:
				return false, fmt.Errorf("invalid symbol %q", read)
			}
		} else {
			nxt = q.onHash
		}
		if nxt == nil {
			return false, fmt.Errorf("missing transition: state %d on %q", q.id, read)
		}

		oldI := i
		move := q.dir
		if move == L {
			i--
		} else {
			i++
		}

		fmt.Printf("%-5d %-10s  %-4s  %-4d %-4s  %d->%d\n",
			step,
			fmt.Sprintf("%d(%s)", q.id, q.dir),
			func() string {
				if i == oldI {
					return "#"
				}
				if oldI < 0 || oldI >= len(tape) {
					return "#"
				}
				return string(read)
			}(),
			nxt.id,
			q.dir,
			oldI, i,
		)
		displayTapeWithHead(tape, i)
		q = nxt
	}
	return false, fmt.Errorf("step cap exceeded (non-halting?)")
}

func parseTapeArg(arg string) (string, error) {
	s := strings.TrimSpace(arg)
	if len(s) >= 2 && s[0] == '#' && s[len(s)-1] == '#' {
		s = s[1 : len(s)-1]
	}
	for i := 0; i < len(s); i++ {
		if s[i] != 'a' && s[i] != 'b' {
			return "", fmt.Errorf("tape must be only a/b (optionally wrapped by #)")
		}
	}
	return s, nil
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

	const stepCap = 10_000_000
	ok, err := runTrace(tape, start, stepCap)
	if err != nil {
		fmt.Println("run error:", err)
		return
	}

	fmt.Printf("Final: #%s#  =>  %s\n", tape, map[bool]string{true: "ACCEPT", false: "REJECT"}[ok])
}
