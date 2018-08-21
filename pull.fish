#!/usr/bin/fish
ls ~/.telegramgo.???????????|awk -F"." '{print $3}' > signed_telephone.txt
set header ""
for x in (cat signed_telephone.txt)
    echo "-------------------------------new_phone_chosen" $x >> pull.txt
    set header (cat pull.txt |grep _text__|tail -1)
    set header (echo $header | awk '{print $1}')
    echo "header_phone_is" $header >> pull.txt
    env header_phone=$header whotele=$x /data/telegramgo.out >> pull.txt
end