package main

import (
	"fmt"
	"time"
)

type PDAMachine struct {
	states []*State
	start  *State
}

func NewPDAMachine(states []*State, start *State) *PDAMachine {
	return &PDAMachine{states: states, start: start}
}

func (m *PDAMachine) Kind() MachineKind { return KindPDA }

func (m *PDAMachine) Dump() { dumpStates(m.states) }

func (m *PDAMachine) WriteDOT(path string) error {
	return writeDOTCommon(m.states, path, false)
}

func (m *PDAMachine) Run(tape []byte) (bool, error) {
	q := m.start
	head := 1
	stack := []byte(nil)
	step := 1

	fmt.Println("== TRACE START ==")
	for {
		if head < 0 || head >= len(tape) {
			return false, fmt.Errorf("PDA head out of bounds: %d", head)
		}
		displayTapeWithHead(string(tape), head)
		cur := tape[head]

		nxt, ok := q.next[cur]
		if !ok {
			fmt.Printf("step=%-4d state=%-3d read=%q  NO-TRANSITION  |S|=%s -> REJECT\n",
				step, q.id, cur, stackToString(stack))
			return false, nil
		}

		fmt.Printf("step=%-4d state=%-3d read=%q next=%-3d head=%-2d\n",
			step, q.id, cur, nxt.id, head)

		switch q.action {
		case ActPush1:
			before := stackToString(stack)
			if q.stackSym == 0 || cur == q.stackSym {
				stack = append(stack, q.stackSym)
				fmt.Printf("    PDA S: PUSH %q  before=%s  after=%s\n",
					q.stackSym, before, stackToString(stack))
			} else {
				fmt.Printf("    PDA S: PUSH skipped (cur=%q != stackSym=%q)  stack=%s\n",
					cur, q.stackSym, before)
			}

		case ActPop1:
			if cur != '#' {
				before := stackToString(stack)
				if len(stack) == 0 {
					fmt.Printf("    PDA S: POP  (stack empty)  before=%s -> REJECT\n", before)
					return false, nil
				}
				top := stack[len(stack)-1]
				if q.stackSym != 0 && top != q.stackSym {
					fmt.Printf("    PDA S: POP  expected=%q got=%q  before=%s -> REJECT\n",
						q.stackSym, top, before)
					return false, nil
				}
				stack = stack[:len(stack)-1]
				fmt.Printf("    PDA S: POP %q  before=%s  after=%s\n",
					top, before, stackToString(stack))
			} else {
				fmt.Printf("    PDA S: POP on '#' -> skipped  stack=%s\n", stackToString(stack))
			}

		default:
			fmt.Printf("    PDA S: (no stack op) stack=%s\n", stackToString(stack))
		}

		if cur != '#' {
			head++
		}

		if nxt.accept {
			// empty-stack accept
			if len(stack) == 0 {
				fmt.Printf("    PDA ACCEPT at state %d with empty stack\n", nxt.id)
				return true, nil
			}
			fmt.Printf("    PDA REJECT at state %d because stack not empty: %s\n",
				nxt.id, stackToString(stack))
			return false, nil
		}
		if nxt.reject {
			fmt.Printf("    PDA REJECT at explicit reject state %d, stack=%s\n",
				nxt.id, stackToString(stack))
			return false, nil
		}

		q = nxt
		step++
		time.Sleep(200 * time.Millisecond)
	}
}
