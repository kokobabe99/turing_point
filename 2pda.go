package main

import (
	"fmt"
	"time"
)

/* ===================== 2-stack PDA Machine ===================== */

type TwoPDAMachine struct {
	states []*State
	start  *State
}

func NewTwoPDAMachine(states []*State, start *State) *TwoPDAMachine {
	return &TwoPDAMachine{states: states, start: start}
}

func (m *TwoPDAMachine) Kind() MachineKind { return KindTwoPDA }

func (m *TwoPDAMachine) Dump() { dumpStates(m.states) }

func (m *TwoPDAMachine) WriteDOT(path string) error {
	return writeDOTCommon(m.states, path, false)
}

func (m *TwoPDAMachine) Run(tape []byte) (bool, error) {
	q := m.start
	head := 1
	var stack1, stack2 []byte
	step := 1

	fmt.Println("== TRACE START ==")
	for {
		if head < 0 || head >= len(tape) {
			return false, fmt.Errorf("2PDA head out of bounds: %d", head)
		}
		displayTapeWithHead(string(tape), head)
		cur := tape[head]

		nxt, ok := q.next[cur]
		if !ok {
			// 没有出边：直接 REJECT
			fmt.Printf("step=%-4d state=%-3d read=%q  NO-TRANSITION  |S1|=%s |S2|=%s -> REJECT\n",
				step, q.id, cur, stackToString(stack1), stackToString(stack2))
			return false, nil
		}

		// 基本信息
		fmt.Printf("step=%-4d state=%-3d read=%q next=%-3d head=%-2d\n",
			step, q.id, cur, nxt.id, head)

		switch q.action {
		case ActPush1:
			before := stackToString(stack1)
			if q.stackSym == 0 || cur == q.stackSym {
				stack1 = append(stack1, q.stackSym)
				fmt.Printf("    2PDA S1: PUSH %q  before=%s  after=%s\n",
					q.stackSym, before, stackToString(stack1))
			} else {
				fmt.Printf("    2PDA S1: PUSH skipped (cur=%q != stackSym=%q)  stack=%s\n",
					cur, q.stackSym, before)
			}

		case ActPop1:
			if cur != '#' {
				before := stackToString(stack1)
				if len(stack1) == 0 {
					fmt.Printf("    2PDA S1: POP (stack empty)  before=%s -> REJECT\n", before)
					return false, nil
				}
				top := stack1[len(stack1)-1]
				if q.stackSym != 0 && top != q.stackSym {
					fmt.Printf("    2PDA S1: POP expected=%q got=%q  before=%s -> REJECT\n",
						q.stackSym, top, before)
					return false, nil
				}
				stack1 = stack1[:len(stack1)-1]
				fmt.Printf("    2PDA S1: POP %q  before=%s  after=%s\n",
					top, before, stackToString(stack1))
			} else {
				fmt.Printf("    2PDA S1: POP on '#' -> skipped  stack=%s\n", stackToString(stack1))
			}

		case ActPush2:
			before := stackToString(stack2)
			if q.stackSym == 0 || cur == q.stackSym {
				stack2 = append(stack2, q.stackSym)
				fmt.Printf("    2PDA S2: PUSH %q  before=%s  after=%s\n",
					q.stackSym, before, stackToString(stack2))
			} else {
				fmt.Printf("    2PDA S2: PUSH skipped (cur=%q != stackSym=%q)  stack=%s\n",
					cur, q.stackSym, before)
			}

		case ActPop2:
			if cur != '#' {
				before := stackToString(stack2)
				if len(stack2) == 0 {
					fmt.Printf("    2PDA S2: POP (stack empty)  before=%s -> REJECT\n", before)
					return false, nil
				}
				top := stack2[len(stack2)-1]
				if q.stackSym != 0 && top != q.stackSym {
					fmt.Printf("    2PDA S2: POP expected=%q got=%q  before=%s -> REJECT\n",
						q.stackSym, top, before)
					return false, nil
				}
				stack2 = stack2[:len(stack2)-1]
				fmt.Printf("    2PDA S2: POP %q  before=%s  after=%s\n",
					top, before, stackToString(stack2))
			} else {
				fmt.Printf("    2PDA S2: POP on '#' -> skipped  stack=%s\n", stackToString(stack2))
			}

		default:
			fmt.Printf("    2PDA S: (no stack op) S1=%s  S2=%s\n",
				stackToString(stack1), stackToString(stack2))
		}

		if cur != '#' {
			head++
		}

		if nxt.accept {
			// 2-stack empty-stack accept
			if len(stack1) == 0 && len(stack2) == 0 {
				fmt.Printf("    2PDA ACCEPT at state %d with S1=%s S2=%s\n",
					nxt.id, stackToString(stack1), stackToString(stack2))
				return true, nil
			}
			fmt.Printf("    2PDA REJECT at state %d because stacks not empty: S1=%s S2=%s\n",
				nxt.id, stackToString(stack1), stackToString(stack2))
			return false, nil
		}
		if nxt.reject {
			fmt.Printf("    2PDA REJECT at explicit reject state %d, S1=%s S2=%s\n",
				nxt.id, stackToString(stack1), stackToString(stack2))
			return false, nil
		}

		q = nxt
		step++
		time.Sleep(200 * time.Millisecond)
	}
}
