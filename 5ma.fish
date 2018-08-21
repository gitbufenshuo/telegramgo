#!/usr/bin/fish
for x in (seq 10000)
    echo now $x
    env only_sign=yes /data/telegramgo.out 1>>5ma.txt
end