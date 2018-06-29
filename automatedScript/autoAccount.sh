#!/bin/bash
 
for i in $(seq 1 150)
do 
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"personal_newAccount","params":["123456"],"id":1}' "http://127.0.0.1:8545"
done
