#!/bin/bash

cd ../cmd/bootnode/

if [ ! -f bootnode.key ];then
	go build
    bootnode -genkey bootnode.key
fi
port=`lsof -i:30301 |awk '{print $2}'|grep -v PID | xargs`

if [ "$port" != "" ];then
	kill $port
fi

## 后台启动bootnode，将输出重定向至bootnode.log文件
nohup bootnode -nodekey=bootnode.key > ../../automatedScript/bootnode/bootnode.log &
exit
