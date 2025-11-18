package main

import (
	"fmt"
	"time"
)

/* ===================== TM Machine ===================== */

type TMMachine struct {
	states []*State
	start  *State
}

func NewTMMachine(states []*State, start *State) *TMMachine {
	return &TMMachine{states: states, start: start}
}

func (m *TMMachine) Kind() MachineKind { return KindTM }

func (m *TMMachine) Dump() { dumpStates(m.states) }

func (m *TMMachine) WriteDOT(path string) error {
	return writeDOTCommon(m.states, path, false)
}

func (m *TMMachine) Run(tape []byte) (bool, error) {
	q := m.start
	head := 1
	step := 1

	fmt.Println("== TRACE START ==")
	for {
		if head < 0 || head >= len(tape) {
			return false, fmt.Errorf("head out of bounds: %d", head)
		}
		displayTapeWithHead(string(tape), head)
		cur := tape[head]

		nxt, ok := q.next[cur]
		if !ok {
			return false, nil
		}

		if q.action == ActWriteTape {
			if q.writeSym != 0 {
				tape[head] = q.writeSym
			}
		}

		fmt.Printf("step  state       read  next  move  head\n")
		fmt.Printf("%-5d %-10s  %-4s  %-4d  %-4s  %d\n",
			step,
			fmt.Sprintf("%d(%s)", q.id, dirStr(q.dir)),
			string(cur),
			nxt.id,
			dirStr(nxt.dir),
			head,
		)

		if nxt.dir == L {
			head--
		} else {
			head++
		}

		if nxt.accept {
			return true, nil
		}
		if nxt.reject {
			return false, nil
		}
		q = nxt
		step++
		time.Sleep(100 * time.Millisecond)
	}
}
