#!/bin/bash

array=($(cat fileName))

balanceKey="balance"
balanceVal=400000000000000000000000000

heftKey="heft"
heftVal=200

stakeKey="stake"
stakeVal=5001
printf "{\n"
printf "\"accounts\":{\n"
for ((i=0;i<${#array[@]};i++))
do
	line=${array[$i]}
	
	num=$(echo $((${#array[@]}-1)))
	if [ "$i" == ${num} ];
	then
        printf "\t\"0x${line##*--}\":{\n"
    	printf "\t\t\"$balanceKey\":\"$balanceVal\",\n"
    	printf "\t\t\"$heftKey\":$heftVal,\n"
    	printf "\t\t\"$stakeKey\":$stakeVal\n"
    	printf "\t}\n"
	else
        printf "\t\"0x${line##*--}\":{\n"
    	printf "\t\t\"$balanceKey\":\"$balanceVal\",\n"
    	printf "\t\t\"$heftKey\":$heftVal,\n"
    	printf "\t\t\"$stakeKey\":$stakeVal\n"
    	printf "\t},\n"
	fi
	let "heftVal=$heftVal+10"
	let "stakeVal=$stakeVal+10"
done
printf "},\n"

printf "\"config\": {
    \"PromissoryNoteEnable\": true,
    \"PromissoryNotePercentage\": 90,
    \"PromissoryNotePrice\":  2000,
    \"LastPromissoryNoteBlockNumber\": 3600,
    \"PromissoryNotePeriod\": 100,
    \"SurplusCoin\": 175000000,
    \"SynStateAccount\": \"0xebb97ad3ca6b4f609da161c0b2b0eaa4ad58f3e8\",
    \"HeftAccount\": \"0xad188b762f9e3ef76c972960b80c9dc99b9cfc73\",
    \"BindingAccount\": \"0xad188b762f9e3ef76c972960b80c9dc99b9cfc73\"
  }\n"

printf "}\n"






#{
#  "0xae606fcc95866f068a8f6750ad56ec14e46e8446":{
#    "balance":"400000000000000000000",
#    "heft":200,
#    "stake":400
#  },
#  "0xc1b2e1fc9d2099f5930a669c9ad65509433550d6":{
#    "balance":"500000000000000000000",
#    "heft":900,
#    "stake":1000
#  },
#  "0x0de2d12fa9c0a5687b330e2de3361e632f52c643":{
#    "balance":"300000000000000000000",
#    "heft":300,
#    "stake":600
#  },
#  "0xad188b762f9e3ef76c972960b80c9dc99b9cfc73":{
#    "balance":"200000000000000000000",
#    "heft":700,
#    "stake":600
#  }
#}
