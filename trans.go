package main

import (
	"fmt"
	"time"
)

/* ===================== Transducer Machine ===================== */

type TransducerMachine struct {
	states []*State
	start  *State
	output []byte
}

func NewTransducerMachine(states []*State, start *State) *TransducerMachine {
	return &TransducerMachine{states: states, start: start}
}

func (m *TransducerMachine) Kind() MachineKind { return KindTransducer }

func (m *TransducerMachine) Dump() { dumpStates(m.states) }

func (m *TransducerMachine) WriteDOT(path string) error {
	// 不显示方向
	return writeDOTCommon(m.states, path, false)
}

func (m *TransducerMachine) Run(tape []byte) (bool, error) {
	m.output = m.output[:0]
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

		// 最后一个 '#' 直接结束
		if cur == '#' && head == len(tape)-1 && q.action != ActPrint {
			fmt.Printf("Output tape: %s\n", string(m.output))
			return true, nil
		}

		// Print 特殊
		if q.action == ActPrint {
			var nxt *State
			if q.next != nil {
				if v, ok := q.next['_']; ok {
					nxt = v
				} else if len(q.next) == 1 {
					for _, v := range q.next {
						nxt = v
						break
					}
				}
			}
			if nxt == nil {
				return false, fmt.Errorf("print state %d missing next", q.id)
			}
			if q.printSym == 0 {
				return false, fmt.Errorf("print state %d has no printSym", q.id)
			}
			m.output = append(m.output, q.printSym)

			fmt.Printf("step  state       read  next  head\n")
			fmt.Printf("%-5d %-10s  %-4s  %-4d  %d\n",
				step,
				fmt.Sprintf("%d", q.id),
				string(cur),
				nxt.id,
				head,
			)

			if nxt.accept {
				fmt.Printf("Output tape: %s\n", string(m.output))
				return true, nil
			}
			if nxt.reject {
				return false, nil
			}
			q = nxt
			step++
			time.Sleep(300 * time.Millisecond)
			continue
		}

		// 普通转移
		nxt, ok := q.next[cur]
		if !ok {
			return false, nil
		}

		fmt.Printf("step  state       read  next  head\n")
		fmt.Printf("%-5d %-10s  %-4s  %-4d  %d\n",
			step,
			fmt.Sprintf("%d", q.id),
			string(cur),
			nxt.id,
			head,
		)

		// transducer: Scan 时遇到非 '#' 才右移
		if q.action == ActScan && cur != '#' {
			head++
		}

		if nxt.accept {
			fmt.Printf("Output tape: %s\n", string(m.output))
			return true, nil
		}
		if nxt.reject {
			return false, nil
		}
		q = nxt
		step++
		time.Sleep(300 * time.Millisecond)
	}
}
