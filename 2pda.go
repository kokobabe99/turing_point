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
	// 2PDA 也不显示方向
	return writeDOTCommon(m.states, path, false)
}

func (m *TwoPDAMachine) Run(tape []byte) (bool, error) {
	q := m.start
	head := 1
	stack1 := []byte(nil)
	stack2 := []byte(nil)
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
			// 无转移：reject
			return false, nil
		}

		// 执行动作
		switch q.action {
		case ActPush1:
			if q.stackSym == 0 || cur == q.stackSym {
				stack1 = append(stack1, q.stackSym)
			}
		case ActPop1:
			if cur != '#' {
				if len(stack1) == 0 {
					return false, fmt.Errorf("2PDA stack1 underflow at state %d", q.id)
				}
				top := stack1[len(stack1)-1]
				if q.stackSym != 0 && top != q.stackSym {
					return false, fmt.Errorf("2PDA stack1 unexpected top %q at state %d", top, q.id)
				}
				stack1 = stack1[:len(stack1)-1]
			}
		case ActPush2:
			if q.stackSym == 0 || cur == q.stackSym {
				stack2 = append(stack2, q.stackSym)
			}
		case ActPop2:
			if cur != '#' {
				if len(stack2) == 0 {
					return false, fmt.Errorf("2PDA stack2 underflow at state %d", q.id)
				}
				top := stack2[len(stack2)-1]
				if q.stackSym != 0 && top != q.stackSym {
					return false, fmt.Errorf("2PDA stack2 unexpected top %q at state %d", top, q.id)
				}
				stack2 = stack2[:len(stack2)-1]
			}
		}

		fmt.Printf("step  state       read  next  head\n")
		fmt.Printf("%-5d %-10s  %-4s  %-4d  %d\n",
			step,
			fmt.Sprintf("%d", q.id),
			string(cur),
			nxt.id,
			head,
		)

		// 2PDA：one-way，只向右扫；遇到 '#' 可以不动
		if cur != '#' {
			head++
		}

		if nxt.accept {
			// 2-stack empty-stack accept
			if len(stack1) == 0 && len(stack2) == 0 {
				return true, nil
			}
			return false, nil
		}
		if nxt.reject {
			return false, nil
		}

		q = nxt
		step++
		time.Sleep(300 * time.Millisecond)
	}
}
