package main

import (
	"fmt"
	"time"
)

/* ===================== TWA Machine ===================== */

type TWAMachine struct {
	states []*State
	start  *State
}

func NewTWAMachine(states []*State, start *State) *TWAMachine {
	return &TWAMachine{states: states, start: start}
}

func (m *TWAMachine) Kind() MachineKind { return KindTWA }

func (m *TWAMachine) Dump() { dumpStates(m.states) }

func (m *TWAMachine) WriteDOT(path string) error {
	return writeDOTCommon(m.states, path, true)
}

func (m *TWAMachine) Run(tape []byte) (bool, error) {
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

		fmt.Printf("step  state       read  next  move  head\n")
		fmt.Printf("%-5d %-10s  %-4s  %-4d  %-4s  %d\n",
			step,
			fmt.Sprintf("%d(%s)", q.id, dirStr(q.dir)),
			string(cur),
			nxt.id,
			dirStr(nxt.dir),
			head,
		)

		if nxt.accept {
			return true, nil
		}
		if nxt.reject {
			return false, nil
		}

		if cur != '#' {
			if q.dir == L {
				head--
			} else {
				head++
			}
		}

		q = nxt
		step++
		time.Sleep(200 * time.Millisecond)
	}
}
