#!/bin/bash

read -t 30 -p "please enter the number of committees(1-120):" committees

if [[ $committees -gt 121 ]];then
	echo "22"
	exit
fi

if [[ $committees -le 0 ]];then
	echo "333"
	exit
fi

cd ../
make geth
cd automatedScript/

ls keystore/* | head -n $committees > fileName

#计数器
i=1

#port端口
port=30315

#rpcport端口
rpcport=8549

rm genaro.json

# rm chainNode

if [ -d "./chainNode" ];then
	rm -r chainNode/*
fi

# rm nohupNodeLog

if [ -d "./nohupNodeLog" ];then
	rm -r nohupNodeLog/*
fi

./generateGenesisJson.sh > ./../cmd/GenGenaroGenesis/genesis.json

cd ../cmd/GenGenaroGenesis/
go build

cp `./GenGenaroGenesis -f genesis.json | xargs` ../../../Genaro-Core/automatedScript/genaro.json

cd ../../../Genaro-Core/automatedScript



./bootnode.sh

sleep 3

if [ ! -f bootnode/bootnode.log ];then
    echo "please run bootnode.sh first"
    exit
fi
bootnode_addr=enode://"$(grep enode bootnode/bootnode.log|tail -n 1|awk -F '://' '{print $2}'|awk -F '@' '{print $1}')""@127.0.0.1:30301"

tmp=`grep enode bootnode/bootnode.log|tail -n 1|awk -F '://' '{print $2}'|awk -F '@' '{print $1}'`
if [ "$tmp" == "" ];then
    echo "node id is empty, please use: bootnode.sh <node_id>";
   	exit
fi


if [ ! -d "./chainNode" ];then
	mkdir ./chainNode
fi

if [ ! -d "./nohupNodeLog" ];then
	mkdir ./nohupNodeLog
fi

#遍历keystore
for line in `cat fileName`
do 
	#kill 端口
	killPort=`lsof -i:$rpcport |awk '{print $2}'|grep -v PID | xargs`
	if [ "$killPort" != "" ];then
    	kill $killPort
	fi	


	#初始化
	./../build/bin/geth  init  ./genaro.json --datadir "./chainNode/chainNode$i"
	
	#key复制到keystore下
	cp $line  ./chainNode/chainNode$i/keystore/${line##*/}
	
	#启动
	nohup ./../build/bin/geth --rpc --rpccorsdomain "*" --rpcvhosts=* --rpcapi "eth,net,web3,admin,personal,miner" --datadir "./chainNode/chainNode$i" --port "$port" --rpcport "$rpcport" --rpcaddr 0.0.0.0  --bootnodes "$bootnode_addr" --unlock "0x${line##*--}" --password "./password"  --syncmode "full" --mine  > ./nohupNodeLog/nohupNode$i.out &
	let "i=$i+1"
	let "port=$port+1"
	let "rpcport=$rpcport+1"
done
