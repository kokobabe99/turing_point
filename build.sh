#!/usr/bin/env bash

echo "========================"
echo " Running ALL automata "
echo "========================"

echo
echo ">>> TWA"
go run main.go twa rules/twa.txt "#abbaabba#"

echo
echo ">>> TM done"
go run main.go tm rules/tm.txt "#aabbaa#"

echo
echo ">>> PDA done"
go run main.go pda rules/pda.txt "#aaabbb#"

echo
echo ">>> 2PDA"
go run main.go 2pda rules/2pda.txt "#aaabbb#"

echo
echo ">>> Transducer done"
go run main.go transducer rules/trans.txt "#000001000010001001011#"

echo
echo "========================"
echo " ALL DONE "
echo "========================"