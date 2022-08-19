#! /usr/bin/env /bin/bash

server=127.0.0.1:8000
odir=results

alg=${1:-fcfs}

declare -a arr=(
    "1us;1us;100;50;2"
    "1us;2us;100;50;2"
    "1us;5us;100;50;2"
    "1us;10us;100;50;2"
    "1us;1us;100;30;2"
    "1us;2us;100;30;2"
    "1us;5us;100;30;2"
    "1us;10us;100;30;2"
)

mkdir -p $odir 
## now loop through the above array
for i in "${arr[@]}"
do
   IFS=';' read -r faint slint n slowpercent conc <<<"$(echo "$i")"
   echo $faint $slint $n $slowpercent $conc

   build/client \
    --slow-request-percent=$slowpercent \
    --slow-request-interval=$slint \
    --fast-request-interval=$faint \
    --concurrency=$conc \
    --n-request=$n \
    --write="${odir}/${alg}_${faint}_${slint}_${n}_${slowpercent}_${conc}"
done